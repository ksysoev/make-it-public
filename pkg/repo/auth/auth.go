package auth

type Repo struct {
	users map[string]string
}

type Config struct {
	Users map[string]string
}

func New(cfg *Config) *Repo {
	return &Repo{
		users: cfg.Users,
	}
}

func (r *Repo) Verify(id, secret string) bool {
	p, ok := r.users[id]
	if !ok {
		return false
	}

	return p == secret
}
