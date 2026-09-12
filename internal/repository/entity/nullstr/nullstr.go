package nullstr

func NilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func OrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
