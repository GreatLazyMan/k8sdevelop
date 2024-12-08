//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by conversion-gen. DO NOT EDIT.

package v1

import (
	config "github.com/GreatLazyMan/simplescheduler/pkg/apis/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func init() {
	localSchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(s *runtime.Scheme) error {
	if err := s.AddGeneratedConversionFunc((*SimplePluginArgs)(nil), (*config.SimplePluginArgs)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1_SimplePluginArgs_To_config_SimplePluginArgs(a.(*SimplePluginArgs), b.(*config.SimplePluginArgs), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.SimplePluginArgs)(nil), (*SimplePluginArgs)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_SimplePluginArgs_To_v1_SimplePluginArgs(a.(*config.SimplePluginArgs), b.(*SimplePluginArgs), scope)
	}); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_SimplePluginArgs_To_config_SimplePluginArgs(in *SimplePluginArgs, out *config.SimplePluginArgs, s conversion.Scope) error {
	if err := metav1.Convert_Pointer_string_To_string(&in.DeploymentName, &out.DeploymentName, s); err != nil {
		return err
	}
	if err := metav1.Convert_Pointer_string_To_string(&in.Image, &out.Image, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1_SimplePluginArgs_To_config_SimplePluginArgs is an autogenerated conversion function.
func Convert_v1_SimplePluginArgs_To_config_SimplePluginArgs(in *SimplePluginArgs, out *config.SimplePluginArgs, s conversion.Scope) error {
	return autoConvert_v1_SimplePluginArgs_To_config_SimplePluginArgs(in, out, s)
}

func autoConvert_config_SimplePluginArgs_To_v1_SimplePluginArgs(in *config.SimplePluginArgs, out *SimplePluginArgs, s conversion.Scope) error {
	if err := metav1.Convert_string_To_Pointer_string(&in.DeploymentName, &out.DeploymentName, s); err != nil {
		return err
	}
	if err := metav1.Convert_string_To_Pointer_string(&in.Image, &out.Image, s); err != nil {
		return err
	}
	return nil
}

// Convert_config_SimplePluginArgs_To_v1_SimplePluginArgs is an autogenerated conversion function.
func Convert_config_SimplePluginArgs_To_v1_SimplePluginArgs(in *config.SimplePluginArgs, out *SimplePluginArgs, s conversion.Scope) error {
	return autoConvert_config_SimplePluginArgs_To_v1_SimplePluginArgs(in, out, s)
}
