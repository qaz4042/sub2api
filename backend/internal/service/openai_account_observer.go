package service

// OpenAIAccountObserver receives request-attempt observations without changing
// gateway behavior. Implementations must return immediately and never retain
// request bodies, tokens or error messages.
type OpenAIAccountObserver interface {
	ObserveOpenAIAccount(account *Account, model, transport string, success, persistentFailure, http5xx bool, firstTokenMs *int)
}

// SetOpenAIAccountObserver is called during server assembly before serving requests.
func (s *OpenAIGatewayService) SetOpenAIAccountObserver(observer OpenAIAccountObserver) {
	s.accountObserver = observer
}
