package digitalocean

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/digitalocean/godo"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"go.woodpecker-ci.org/autoscaler/config"
	"go.woodpecker-ci.org/autoscaler/engine/inits/cloudinit"
	"go.woodpecker-ci.org/autoscaler/engine/types"
	"go.woodpecker-ci.org/woodpecker/v3/woodpecker-go/woodpecker"
)

var (
	ErrParameterNotSet = errors.New("required parameter not set")
	ErrSSHKeyNotFound  = errors.New("SSH key not found")
)

type Provider struct {
	name             string
	region           string
	size             string
	image            string
	tags             []string
	poolTag          string
	vpcUUID          string
	enableIPv6       bool
	enableMonitoring bool
	sshKeySelectors  []string
	sshKeys          []godo.DropletCreateSSHKey
	userDataTemplate *template.Template
	config           *config.Config
	client           *godo.Client
}

func New(ctx context.Context, c *cli.Command, config *config.Config) (types.Provider, error) {
	apiToken := c.String("digitalocean-api-token")
	if apiToken == "" {
		return nil, fmt.Errorf("%w: digitalocean-api-token", ErrParameterNotSet)
	}

	p := &Provider{
		name:             "digitalocean",
		region:           c.String("digitalocean-region"),
		size:             c.String("digitalocean-size"),
		image:            c.String("digitalocean-image"),
		tags:             c.StringSlice("digitalocean-tags"),
		poolTag:          poolTag(config.PoolID),
		vpcUUID:          c.String("digitalocean-vpc-uuid"),
		enableIPv6:       c.Bool("digitalocean-enable-ipv6"),
		enableMonitoring: c.Bool("digitalocean-enable-monitoring"),
		sshKeySelectors:  c.StringSlice("digitalocean-ssh-keys"),
		config:           config,
		client:           godo.NewFromToken(apiToken),
	}
	p.tags = mergeTags([]string{"woodpecker-autoscaler", p.poolTag}, p.tags)

	err := p.setupKeypair(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: setupKeypair: %w", p.name, err)
	}

	// # TODO: Deprecated remove in v2.0
	if u := c.String("digitalocean-user-data"); u != "" {
		log.Warn().Msg("digitalocean-user-data is deprecated, please use provider-user-data instead")
		userDataTmpl, err := template.New("user-data").Parse(u)
		if err != nil {
			return nil, fmt.Errorf("%s: template.New.Parse %w", p.name, err)
		}
		p.userDataTemplate = userDataTmpl
	}

	return p, nil
}

func (p *Provider) DeployAgent(ctx context.Context, agent *woodpecker.Agent) error {
	userData, err := cloudinit.RenderUserDataTemplate(p.config, agent, p.userDataTemplate)
	if err != nil {
		return fmt.Errorf("%s: cloudinit.RenderUserDataTemplate: %w", p.name, err)
	}

	req := &godo.DropletCreateRequest{
		Name:       agent.Name,
		Region:     p.region,
		Size:       p.size,
		Image:      godo.DropletCreateImage{Slug: p.image},
		SSHKeys:    p.sshKeys,
		IPv6:       p.enableIPv6,
		Monitoring: p.enableMonitoring,
		UserData:   userData,
		Tags:       p.tags,
		VPCUUID:    p.vpcUUID,
	}

	droplet, _, err := p.client.Droplets.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("%s: Droplets.Create: %w", p.name, err)
	}

	log.Debug().Msgf("waiting for droplet %d", droplet.ID)
	for range 5 {
		agents, err := p.ListDeployedAgentNames(ctx)
		if err != nil {
			return fmt.Errorf("failed to return list for agents")
		}

		for _, a := range agents {
			if a == agent.Name {
				return nil
			}
		}

		log.Debug().Msgf("created agent not found in list yet")
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("droplet did not resolve in agent list: %d", droplet.ID)
}

func (p *Provider) RemoveAgent(ctx context.Context, agent *woodpecker.Agent) error {
	droplet, err := p.getAgent(ctx, agent.Name)
	if err != nil {
		return fmt.Errorf("%s: %w", p.name, err)
	}

	if droplet == nil {
		return nil
	}

	_, err = p.client.Droplets.Delete(ctx, droplet.ID)
	if err != nil {
		return fmt.Errorf("%s: Droplets.Delete: %w", p.name, err)
	}

	return nil
}

func (p *Provider) ListDeployedAgentNames(ctx context.Context) ([]string, error) {
	droplets, err := p.listDropletsByTag(ctx, p.poolTag)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(droplets))
	for _, droplet := range droplets {
		names = append(names, droplet.Name)
	}

	return names, nil
}

func (p *Provider) getAgent(ctx context.Context, name string) (*godo.Droplet, error) {
	droplets, err := p.listDropletsByName(ctx, name)
	if err != nil {
		return nil, err
	}

	matches := make([]godo.Droplet, 0, len(droplets))
	for _, droplet := range droplets {
		if droplet.Name == name && containsTag(droplet.Tags, p.poolTag) {
			matches = append(matches, droplet)
		}
	}

	if len(matches) == 0 {
		return nil, nil
	}

	if len(matches) > 1 {
		return nil, fmt.Errorf("found multiple droplets with name %s", name)
	}

	return &matches[0], nil
}

func (p *Provider) listDropletsByName(ctx context.Context, name string) ([]godo.Droplet, error) {
	var droplets []godo.Droplet
	opt := &godo.ListOptions{
		Page:    1,
		PerPage: 200, //nolint:mnd
	}

	for {
		page, resp, err := p.client.Droplets.ListByName(ctx, name, opt)
		if err != nil {
			return nil, fmt.Errorf("%s: Droplets.ListByName: %w", p.name, err)
		}
		droplets = append(droplets, page...)

		if resp == nil || resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		opt.Page++
	}

	return droplets, nil
}

func (p *Provider) listDropletsByTag(ctx context.Context, tag string) ([]godo.Droplet, error) {
	var droplets []godo.Droplet
	opt := &godo.ListOptions{
		Page:    1,
		PerPage: 200, //nolint:mnd
	}

	for {
		page, resp, err := p.client.Droplets.ListByTag(ctx, tag, opt)
		if err != nil {
			return nil, fmt.Errorf("%s: Droplets.ListByTag: %w", p.name, err)
		}
		droplets = append(droplets, page...)

		if resp == nil || resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		opt.Page++
	}

	return droplets, nil
}

func (p *Provider) setupKeypair(ctx context.Context) error {
	keys, err := p.listKeys(ctx)
	if err != nil {
		return err
	}

	for _, selector := range p.sshKeySelectors {
		key, ok := resolveSSHKey(selector, keys)
		if !ok {
			return fmt.Errorf("%w: %s", ErrSSHKeyNotFound, selector)
		}
		p.sshKeys = append(p.sshKeys, key)
	}
	if len(p.sshKeys) > 0 {
		return nil
	}

	for _, name := range []string{"woodpecker", "id_rsa_woodpecker"} {
		key, ok := resolveSSHKey(name, keys)
		if !ok {
			continue
		}
		p.sshKeys = append(p.sshKeys, key)

		return nil
	}

	if len(keys) > 0 {
		p.sshKeys = append(p.sshKeys, dropletCreateSSHKey(keys[0]))
		return nil
	}

	return ErrSSHKeyNotFound
}

func (p *Provider) listKeys(ctx context.Context) ([]godo.Key, error) {
	var keys []godo.Key
	opt := &godo.ListOptions{
		Page:    1,
		PerPage: 200, //nolint:mnd
	}

	for {
		page, resp, err := p.client.Keys.List(ctx, opt)
		if err != nil {
			return nil, fmt.Errorf("%s: Keys.List: %w", p.name, err)
		}
		keys = append(keys, page...)

		if resp == nil || resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		opt.Page++
	}

	return keys, nil
}

func resolveSSHKey(selector string, keys []godo.Key) (godo.DropletCreateSSHKey, bool) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return godo.DropletCreateSSHKey{}, false
	}

	if id, err := strconv.Atoi(selector); err == nil && id > 0 {
		return godo.DropletCreateSSHKey{ID: id}, true
	}

	for _, key := range keys {
		if selector == key.Fingerprint || selector == key.Name {
			return dropletCreateSSHKey(key), true
		}
	}

	return godo.DropletCreateSSHKey{}, false
}

func dropletCreateSSHKey(key godo.Key) godo.DropletCreateSSHKey {
	if key.Fingerprint != "" {
		return godo.DropletCreateSSHKey{Fingerprint: key.Fingerprint}
	}

	return godo.DropletCreateSSHKey{ID: key.ID}
}

func poolTag(poolID string) string {
	const prefix = "woodpecker-pool-"

	slug := sanitizeTagPart(poolID)
	if slug == "" {
		slug = "default"
	}

	if len(prefix)+len(slug) > 255 {
		slug = slug[:255-len(prefix)]
	}

	return prefix + slug
}

func sanitizeTagPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))

	var out strings.Builder
	lastDash := false
	for _, r := range value {
		valid := r >= 'a' && r <= 'z' ||
			r >= '0' && r <= '9' ||
			r == '-' ||
			r == '_' ||
			r == ':'
		if !valid {
			if !lastDash {
				out.WriteByte('-')
				lastDash = true
			}
			continue
		}
		out.WriteRune(r)
		lastDash = r == '-'
	}

	return strings.Trim(out.String(), "-")
}

func mergeTags(defaults, extra []string) []string {
	seen := make(map[string]struct{}, len(defaults)+len(extra))
	tags := make([]string, 0, len(defaults)+len(extra))

	for _, tag := range append(defaults, extra...) {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}

	return tags
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}

	return false
}
