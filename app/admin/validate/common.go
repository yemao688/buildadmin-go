package validate

type IDS struct {
	ID int64 `uri:"id" form:"id" json:"id" binding:"required"`
}

func (v IDS) GetMessages() ValidatorMessages {
	return ValidatorMessages{
		"id.required": "ID不能为空",
	}
}

type ID32S struct {
	ID int32 `uri:"id" form:"id" json:"id" binding:"required"`
}

func (v ID32S) GetMessages() ValidatorMessages {
	return ValidatorMessages{
		"id.required": "ID不能为空",
	}
}

type IDStr struct {
	ID string `uri:"id" form:"id" json:"id" binding:"required"`
}

func (v IDStr) GetMessages() ValidatorMessages {
	return ValidatorMessages{
		"id.required": "ID不能为空",
	}
}

type StatusS struct {
	Status *int32 `form:"status" json:"status" binding:"omitempty,numeric" `
}

func (v StatusS) GetMessages() ValidatorMessages {
	return ValidatorMessages{
		"status.numeric": "状态值必须数字",
	}
}

type PaginationS struct {
	Size int `form:"size" json:"size" binding:"omitempty,gte=1,lte=10000,number"`
	Page int `form:"page" json:"page" binding:"omitempty,number"`
}

func (v PaginationS) GetMessages() ValidatorMessages {
	return ValidatorMessages{
		"size.gte":    "size必须大于等于1",
		"size.lte":    "size不能超过1万",
		"size.number": "size必须数字",
		"page.number": "页数必须数字",
	}
}

// 日期时间字符串 "2006-01-02 15:04:05"
type TimeS struct {
	StartTime string `form:"start_time" json:"start_time"`
	EndTime   string `form:"end_time" json:"end_time"`
}

func (v TimeS) GetMessages() ValidatorMessages {
	return ValidatorMessages{}
}

// 日期字符串 "2006-01-02"
type DayS struct {
	StartDay string `form:"start_day" json:"start_day"`
	EndDay   string `form:"end_day" json:"end_day"`
}

func (v DayS) GetMessages() ValidatorMessages {
	return ValidatorMessages{}
}

// 日期字符串数组 ["2006-01-02","2006-01-02"]
type DateS struct {
	Date []string `form:"date[]" json:"date[]"`
}

func (v DateS) GetMessages() ValidatorMessages {
	return ValidatorMessages{}
}

type SendCode struct {
	Phone string `form:"phone" json:"phone" binding:"required,phone"`
	Way   string `form:"way" json:"way" binding:"required,oneof=register login verify"`
}

func (v SendCode) GetMessages() ValidatorMessages {
	return ValidatorMessages{
		"phone.required": "手机号码不能为空",
		"phone.phone":    "手机号码格式不正确",
		"way.required":   "请求类型必须",
		"way.oneof":      "请求类型不正确",
	}
}

type Name struct {
	Name string `uri:"name" form:"name" json:"name" `
}
