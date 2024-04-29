package validate

type Ids struct {
	Ids []int64 `json:"ids" binding:"required"`
}

func (v Ids) GetMessages() ValidatorMessages {
	return ValidatorMessages{
		"ids.required": "ID不能为空",
	}
}
