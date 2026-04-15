package types

import "testing"

func TestServingRuntimeOverrideConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *ServingRuntimeOverrideConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name:    "empty config",
			config:  &ServingRuntimeOverrideConfig{},
			wantErr: true,
		},
		{
			name: "missing preview_image",
			config: &ServingRuntimeOverrideConfig{
				Reason:      "test reason",
				RuntimeName: "test-runtime",
				DisplayName: "Test Runtime",
			},
			wantErr: true,
		},
		{
			name: "missing runtime_name",
			config: &ServingRuntimeOverrideConfig{
				PreviewImage: "registry.example.com/image:tag",
				Reason:       "test reason",
				DisplayName:  "Test Runtime",
			},
			wantErr: true,
		},
		{
			name: "missing display_name",
			config: &ServingRuntimeOverrideConfig{
				PreviewImage: "registry.example.com/image:tag",
				Reason:       "test reason",
				RuntimeName:  "test-runtime",
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &ServingRuntimeOverrideConfig{
				PreviewImage: "registry.example.com/image:tag",
				Reason:       "test reason",
				RuntimeName:  "test-runtime",
				DisplayName:  "Test Runtime",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
