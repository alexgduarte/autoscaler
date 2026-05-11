package digitalocean

import (
	"os"

	"github.com/urfave/cli/v3"
)

const category = "DigitalOcean"

var ProviderFlags = []cli.Flag{
	&cli.StringFlag{
		Name:  "digitalocean-api-token",
		Usage: "DigitalOcean API token",
		Sources: cli.NewValueSourceChain(
			cli.EnvVar("WOODPECKER_DIGITALOCEAN_API_TOKEN"),
			cli.File(os.Getenv("WOODPECKER_DIGITALOCEAN_API_TOKEN_FILE")),
		),
		Category: category,
	},
	&cli.StringFlag{
		Name:     "digitalocean-region",
		Value:    "nyc3",
		Usage:    "DigitalOcean region slug",
		Sources:  cli.EnvVars("WOODPECKER_DIGITALOCEAN_REGION"),
		Category: category,
	},
	&cli.StringFlag{
		Name:     "digitalocean-size",
		Value:    "s-1vcpu-1gb",
		Usage:    "DigitalOcean droplet size slug",
		Sources:  cli.EnvVars("WOODPECKER_DIGITALOCEAN_SIZE"),
		Category: category,
	},
	&cli.StringFlag{
		Name:     "digitalocean-image",
		Value:    "ubuntu-22-04-x64",
		Usage:    "DigitalOcean image slug",
		Sources:  cli.EnvVars("WOODPECKER_DIGITALOCEAN_IMAGE"),
		Category: category,
	},
	&cli.StringSliceFlag{
		Name:     "digitalocean-ssh-keys",
		Usage:    "DigitalOcean SSH key IDs, fingerprints, or names",
		Sources:  cli.EnvVars("WOODPECKER_DIGITALOCEAN_SSH_KEYS"),
		Category: category,
	},
	&cli.StringSliceFlag{
		Name:     "digitalocean-tags",
		Usage:    "additional DigitalOcean droplet tags",
		Sources:  cli.EnvVars("WOODPECKER_DIGITALOCEAN_TAGS"),
		Category: category,
	},
	&cli.StringFlag{
		Name:     "digitalocean-vpc-uuid",
		Usage:    "DigitalOcean VPC UUID for created droplets",
		Sources:  cli.EnvVars("WOODPECKER_DIGITALOCEAN_VPC_UUID"),
		Category: category,
	},
	&cli.BoolFlag{
		Name:     "digitalocean-enable-ipv6",
		Value:    true,
		Usage:    "enable public IPv6 for created droplets",
		Sources:  cli.EnvVars("WOODPECKER_DIGITALOCEAN_ENABLE_IPV6"),
		Category: category,
	},
	&cli.BoolFlag{
		Name:     "digitalocean-enable-monitoring",
		Value:    true,
		Usage:    "enable DigitalOcean monitoring for created droplets",
		Sources:  cli.EnvVars("WOODPECKER_DIGITALOCEAN_ENABLE_MONITORING"),
		Category: category,
	},
	// TODO: Deprecated remove in v2.0
	&cli.StringFlag{
		Name:  "digitalocean-user-data (deprecated)",
		Usage: "DigitalOcean userdata template",
		Sources: cli.NewValueSourceChain(
			cli.EnvVar("WOODPECKER_DIGITALOCEAN_USERDATA"),
			cli.File(os.Getenv("WOODPECKER_DIGITALOCEAN_USERDATA_FILE")),
		),
		Category: category,
	},
}
