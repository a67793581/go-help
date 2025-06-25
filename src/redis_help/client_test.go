package redis_help

import (
	"testing"
)

func TestNewRedis(t *testing.T) {
	type args struct {
		config *DataRedis
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Valid Cluster Configuration",
			args: args{
				config: &DataRedis{
					Address:      "localhost:6379",
					IsCluster:    true,
					ReadTimeout:  Duration(5),
					WriteTimeout: Duration(5),
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid Address",
			args: args{
				config: &DataRedis{
					Address: "",
				},
			},
			wantErr: true,
		},
		{
			name: "Single Node Configuration",
			args: args{
				config: &DataRedis{
					Address:      "localhost:6379",
					IsCluster:    false,
					ReadTimeout:  Duration(5),
					WriteTimeout: Duration(5),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewRedis(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRedis() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("NewRedis() returned nil client")
			}
		})
	}
}

func TestRegisterCache(t *testing.T) {
	type args struct {
		configs []DataRedis
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Multiple Redis Configurations",
			args: args{
				configs: []DataRedis{
					{
						Alias:        "cluster1",
						Address:      "localhost:6379",
						IsCluster:    false,
						ReadTimeout:  Duration(5),
						WriteTimeout: Duration(5),
					},
					{
						Alias:        "single1",
						Address:      "localhost:6379",
						IsCluster:    false,
						ReadTimeout:  Duration(5),
						WriteTimeout: Duration(5),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Empty Address",
			args: args{
				configs: []DataRedis{
					{
						Alias:   "test",
						Address: "",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Empty Alias",
			args: args{
				configs: []DataRedis{
					{
						Alias:   "",
						Address: "localhost:6379",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RegisterCache(tt.args.configs)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterCache() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.args.configs) {
					t.Errorf("RegisterCache() returned %d clients, expected %d", len(got), len(tt.args.configs))
				}
				for _, config := range tt.args.configs {
					if _, exists := got[config.Alias]; !exists {
						t.Errorf("RegisterCache() missing client for alias %s", config.Alias)
					}
				}
			}
		})
	}
}
