package validate

type AdminRule struct {
}

func (v AdminRule) GetMessages() ValidatorMessages {
	return ValidatorMessages{}
}
