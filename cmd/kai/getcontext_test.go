package main

import "testing"

func TestParseDeployTarget(t *testing.T) {
	tests := []struct {
		arg     string
		want    string
		wantErr bool
	}{
		{arg: "deploy/checkout-service", want: "checkout-service"},
		{arg: "deployment/checkout-service", want: "checkout-service"},
		{arg: "Deployments/checkout-service", want: "checkout-service"},
		{arg: "checkout-service", wantErr: true},
		{arg: "deploy/", wantErr: true},
		{arg: "pod/checkout-service-abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			got, err := parseDeployTarget(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseDeployTarget(%q) error = %v, wantErr %v", tt.arg, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseDeployTarget(%q) = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}
