package validate

type Admin struct {
}

func (v Admin) GetMessages() ValidatorMessages {
	return ValidatorMessages{}
}
