package types

import "testing"

func TestMCPServerMetadata_Validate(t *testing.T) {
	tests := []struct {
		name    string
		server  *MCPServerMetadata
		wantErr bool
	}{
		{
			name:    "nil receiver is valid",
			server:  nil,
			wantErr: false,
		},
		{
			name:    "valid name and version",
			server:  &MCPServerMetadata{Name: "com.redhat/openshift-mcp-server", Version: "0.4.0"},
			wantErr: false,
		},
		{
			name:    "valid name with three namespace labels",
			server:  &MCPServerMetadata{Name: "io.github.confluentinc/mcp-confluent", Version: "1.0.0"},
			wantErr: false,
		},
		{
			name:    "valid name with digits in namespace",
			server:  &MCPServerMetadata{Name: "com.web3/tool", Version: "1.0.0"},
			wantErr: false,
		},
		{
			name:    "valid name with dot in slug",
			server:  &MCPServerMetadata{Name: "org.mariadb/mcp.server", Version: "1.0.0"},
			wantErr: false,
		},
		{
			name:    "valid name matching real mariadb production entry",
			server:  &MCPServerMetadata{Name: "org.mariadb/mcp", Version: "1.0.0"},
			wantErr: false,
		},
		{
			name:    "valid name with hyphen in namespace label",
			server:  &MCPServerMetadata{Name: "com.my-company/their-server", Version: "1.0.0"},
			wantErr: false,
		},
		{
			name:    "valid name with single-character slug",
			server:  &MCPServerMetadata{Name: "com.example/x", Version: "1.0.0"},
			wantErr: false,
		},
		{
			name:    "valid prerelease version",
			server:  &MCPServerMetadata{Name: "com.redhat/openshift-mcp-server", Version: "2.0.0-beta"},
			wantErr: false,
		},
		{
			name:    "valid prerelease with dotted identifiers",
			server:  &MCPServerMetadata{Name: "com.redhat/openshift-mcp-server", Version: "0.1.0-rc.1"},
			wantErr: false,
		},
		{
			name:    "valid version with build metadata",
			server:  &MCPServerMetadata{Name: "com.redhat/openshift-mcp-server", Version: "1.5.0-betaexp.sha.5114f85"},
			wantErr: false,
		},
		{
			name:    "valid version with plus build metadata",
			server:  &MCPServerMetadata{Name: "com.redhat/openshift-mcp-server", Version: "1.0.0+build.123"},
			wantErr: false,
		},
		{
			name:    "name missing namespace",
			server:  &MCPServerMetadata{Name: "openshift-mcp-server", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "name with single-label namespace",
			server:  &MCPServerMetadata{Name: "redhat/openshift-mcp-server", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "name with uppercase characters",
			server:  &MCPServerMetadata{Name: "COM.RedHat/openshift-mcp-server", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "empty name",
			server:  &MCPServerMetadata{Name: "", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "name with empty slug",
			server:  &MCPServerMetadata{Name: "com.redhat/", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "name with empty namespace",
			server:  &MCPServerMetadata{Name: "/openshift-mcp-server", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "name with underscore in slug",
			server:  &MCPServerMetadata{Name: "com.redhat/my_server", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "name with leading hyphen in namespace label",
			server:  &MCPServerMetadata{Name: "com.-bad/server", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "name with trailing hyphen in namespace label",
			server:  &MCPServerMetadata{Name: "com.bad-/server", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "version is latest",
			server:  &MCPServerMetadata{Name: "com.redhat/openshift-mcp-server", Version: "latest"},
			wantErr: true,
		},
		{
			name:    "version missing patch component",
			server:  &MCPServerMetadata{Name: "com.redhat/openshift-mcp-server", Version: "0.4"},
			wantErr: true,
		},
		{
			name:    "version is a two-component product version",
			server:  &MCPServerMetadata{Name: "com.redhat/satellite-mcp-server", Version: "6.18"},
			wantErr: true,
		},
		{
			name:    "empty version",
			server:  &MCPServerMetadata{Name: "com.redhat/openshift-mcp-server", Version: ""},
			wantErr: true,
		},
		{
			name:    "version with v prefix",
			server:  &MCPServerMetadata{Name: "com.redhat/openshift-mcp-server", Version: "v1.0.0"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.server.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
