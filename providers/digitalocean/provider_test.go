package digitalocean

import (
	"testing"

	"github.com/digitalocean/godo"
	"github.com/stretchr/testify/require"
)

func TestResolveSSHKey(t *testing.T) {
	keys := []godo.Key{
		{ID: 1, Name: "woodpecker", Fingerprint: "aa:bb"},
		{ID: 2, Name: "fallback"},
	}

	key, ok := resolveSSHKey("42", keys)
	require.True(t, ok)
	require.Equal(t, 42, key.ID)

	key, ok = resolveSSHKey("woodpecker", keys)
	require.True(t, ok)
	require.Equal(t, "aa:bb", key.Fingerprint)

	key, ok = resolveSSHKey("aa:bb", keys)
	require.True(t, ok)
	require.Equal(t, "aa:bb", key.Fingerprint)

	key, ok = resolveSSHKey("fallback", keys)
	require.True(t, ok)
	require.Equal(t, 2, key.ID)

	_, ok = resolveSSHKey("missing", keys)
	require.False(t, ok)
}

func TestPoolTag(t *testing.T) {
	require.Equal(t, "woodpecker-pool-prod-eu-1", poolTag("Prod/EU 1"))
	require.Equal(t, "woodpecker-pool-default", poolTag(" / "))
	require.Equal(t, "woodpecker-pool-prod:europe", poolTag("prod:europe"))
}

func TestMergeTags(t *testing.T) {
	tags := mergeTags([]string{"woodpecker-autoscaler", "woodpecker-pool-1"}, []string{"", "custom", "woodpecker-pool-1"})

	require.Equal(t, []string{"woodpecker-autoscaler", "woodpecker-pool-1", "custom"}, tags)
}
