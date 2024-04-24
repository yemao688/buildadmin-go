package validate

type TestBuild struct {
}

func (v TestBuild) GetMessages() ValidatorMessages {
	return ValidatorMessages{}
}
