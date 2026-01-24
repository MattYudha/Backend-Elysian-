package cache

import "fmt"

type CacheKeyBuilder struct {
	prefix string
}

func NewCacheKeyBuilder(prefix string) *CacheKeyBuilder {
	return &CacheKeyBuilder{
		prefix: prefix,
	}
}

func (b *CacheKeyBuilder) UserByID(id string) string {
	return fmt.Sprintf("%s:user:id:%s", b.prefix, id)
}

func (b *CacheKeyBuilder) UserByEmail(email string) string {
	return fmt.Sprintf("%s:user:email:%s", b.prefix, email)
}

func (b *CacheKeyBuilder) Session(sessionID string) string {
	return fmt.Sprintf("%s:session:%s", b.prefix, sessionID)
}

func (b *CacheKeyBuilder) RefreshToken(token string) string {
	return fmt.Sprintf("%s:refresh_token:%s", b.prefix, token)
}

func (b *CacheKeyBuilder) Workflow(id string) string {
	return fmt.Sprintf("%s:workflow:%s", b.prefix, id)
}

func (b *CacheKeyBuilder) WorkflowList(userID string, page int) string {
	return fmt.Sprintf("%s:workflow:list:%s:page:%d", b.prefix, userID, page)
}

func (b *CacheKeyBuilder) Execution(id string) string {
	return fmt.Sprintf("%s:execution:%s", b.prefix, id)
}

func (b *CacheKeyBuilder) RateLimit(identifier string) string {
	return fmt.Sprintf("%s:rate_limit:%s", b.prefix, identifier)
}

func (b *CacheKeyBuilder) Custom(parts ...string) string {
	key := b.prefix
	for _, part := range parts {
		key = fmt.Sprintf("%s:%s", key, part)
	}

	return key
}
