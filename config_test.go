package main

import "testing"

func Test_validateConfig(t *testing.T) {
	type args struct {
		config Config
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"empty_path",
			args{Config{WebHooks: []*WebHook{
				{Path: ""},
			}}},
			true,
		}, {
			"with_out_/",
			args{Config{WebHooks: []*WebHook{
				{Path: "path"},
			}}},
			true,
		},
		{
			"valid",
			args{Config{WebHooks: []*WebHook{
				{Path: "/wh1"},
				{Path: "/wh2"},
				{Path: "/path/wh3"},
			}}},
			false,
		},
		{
			"duplicate_paths",
			args{Config{WebHooks: []*WebHook{
				{Path: "/wh"},
				{Path: "/wh"},
				{Path: "/path/wh3"},
			}}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateConfig(tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
