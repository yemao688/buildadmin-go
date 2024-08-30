package validate

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	. "github.com/go-playground/assert/v2"
	"github.com/go-playground/validator/v10"
)

type Param struct {
	Date  string            `validate:"omitempty,required,datetime=2006-01-02"`
	Array []string          `validate:"required,gt=0,dive,required"`
	Map   map[string]string `validate:"required,gt=0,dive,keys,max=5,endkeys,required,max=1000"`
}

func TestValide(t *testing.T) {
	v := validator.New()

	data := Param{
		Date: "2006-01-02 12:22:22",
	}

	data.Array = []string{"test"}
	data.Map = map[string]string{"test": "test"}

	err := v.Struct(data)
	if err != nil {
		fmt.Println(err)
	}
}

func TestRequired(t *testing.T) {
	validate := validator.New()
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]

		if name == "-" {
			return ""
		}

		return name
	})

	data := struct {
		Type  string `json:"type" validate:"required"`
		Email string `json:"email" validate:"required_if=Type email,omitempty,email"`
	}{
		Type:  "email",
		Email: "ww@qq.com",
	}

	err := validate.Struct(data)
	if err != nil {
		fmt.Println(err)
	}
}

func TestTime(t *testing.T) {
	validate := validator.New()
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]

		if name == "-" {
			return ""
		}

		return name
	})

	type TestTime struct {
		Time string `validate:"required"`
	}

	var testTime TestTime

	err := validate.Struct(&testTime)
	// fmt.Println(err)
	NotEqual(t, err, nil)
	AssertError(t, err.(validator.ValidationErrors), "TestTime.Time", "TestTime.Time", "Time", "Time", "required")
}

func AssertError(t *testing.T, err error, nsKey, structNsKey, field, structField, expectedTag string) {
	errs := err.(validator.ValidationErrors)

	found := false
	var fe validator.FieldError

	for i := 0; i < len(errs); i++ {
		fmt.Println(errs[i].Namespace())
		fmt.Println(errs[i].StructNamespace())

		if errs[i].Namespace() == nsKey && errs[i].StructNamespace() == structNsKey {
			found = true
			fe = errs[i]
			break
		}
	}

	// fmt.Println(fe.Field())
	// fmt.Println(fe.StructField())
	// fmt.Println(fe.Tag())

	EqualSkip(t, 2, found, true)
	NotEqualSkip(t, 2, fe, nil)
	EqualSkip(t, 2, fe.Field(), field)
	EqualSkip(t, 2, fe.StructField(), structField)
	EqualSkip(t, 2, fe.Tag(), expectedTag)
}
