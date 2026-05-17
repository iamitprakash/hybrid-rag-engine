package tenantauth

import "strings"

type Auth struct {
	apiKeys map[string]string
}

func New(config string) *Auth {
	auth := &Auth{apiKeys: map[string]string{}}
	for _, pair := range strings.Split(config, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}
		tenant := strings.TrimSpace(parts[0])
		apiKey := strings.TrimSpace(parts[1])
		if tenant == "" || apiKey == "" {
			continue
		}
		auth.apiKeys[apiKey] = tenant
	}
	return auth
}

func (a *Auth) Enabled() bool {
	return a != nil && len(a.apiKeys) > 0
}

func (a *Auth) TenantForKey(apiKey string) (string, bool) {
	tenant, ok := a.apiKeys[apiKey]
	return tenant, ok
}
