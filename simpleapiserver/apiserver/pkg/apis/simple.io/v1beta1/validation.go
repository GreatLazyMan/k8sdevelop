package v1beta1

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateFoo validates a Foo
func ValidateFoo(f *Foo) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateFooSpec(&f.Spec, field.NewPath("spec"))...)

	return allErrs
}

func ValidateFooSpec(s *FooSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(s.Image) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("image"), ""))
	} else if len(s.Config.Msg) == 0 {
		allErrs = append(allErrs, ValidateFooSpecConfig(&s.Config, fldPath.Child("config"))...)
	}

	return allErrs
}

func ValidateFooSpecConfig(cfg *FooConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(cfg.Msg) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("msg"), ""))
	}
	return allErrs
}
