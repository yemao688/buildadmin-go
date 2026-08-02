package validate

type Ids struct {
	Ids []int32 `form:"ids[]" binding:"required"`
}

func (v Ids) GetMessages() ValidatorMessages {
	return ValidatorMessages{
		"ids.required": "ID不能为空",
	}
}
