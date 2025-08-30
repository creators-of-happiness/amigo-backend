package util

func LooksLikePhone(p string) bool {
	for _, r := range p {
		if !(r == '+' || r == ' ' || r == '-' || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return len(p) >= 6
}
