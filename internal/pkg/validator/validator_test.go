package validator

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	. "github.com/go-playground/assert/v2"
	"github.com/go-playground/validator/v10"
)

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
