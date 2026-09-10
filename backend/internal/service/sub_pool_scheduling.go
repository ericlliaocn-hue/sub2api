package service

// Sub-pool membership is injected after construction rather than through the
// gateway constructors: it is an optional isolation layer, and a nil resolver
// must keep scheduling exactly as it behaved before sub-pools existed.

// SetSubPoolMembership attaches the sub-pool resolver used to narrow scheduling
// candidates to the API key's pool.
func (s *GatewayService) SetSubPoolMembership(m *SubPoolMembership) {
	if s == nil {
		return
	}
	s.subPoolMembership = m
}

// SetSubPoolMembership attaches the sub-pool resolver used to narrow scheduling
// candidates to the API key's pool.
func (s *OpenAIGatewayService) SetSubPoolMembership(m *SubPoolMembership) {
	if s == nil {
		return
	}
	s.subPoolMembership = m
}

// SetSubPoolMembership attaches the sub-pool resolver used to narrow scheduling
// candidates to the API key's pool.
func (s *GeminiMessagesCompatService) SetSubPoolMembership(m *SubPoolMembership) {
	if s == nil {
		return
	}
	s.subPoolMembership = m
}
