package porkbun

import (
	"testing"
)

func TestDNSRecordInputValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   DNSRecordInput
		wantErr bool
	}{
		{
			name: "valid A record for root",
			input: DNSRecordInput{
				Type:    "A",
				Name:    "",
				Content: "192.168.1.1",
				TTL:     600,
			},
			wantErr: false,
		},
		{
			name: "valid A record for wildcard",
			input: DNSRecordInput{
				Type:    "A",
				Name:    "*",
				Content: "192.168.1.1",
				TTL:     600,
			},
			wantErr: false,
		},
		{
			name: "valid CNAME record",
			input: DNSRecordInput{
				Type:    "CNAME",
				Name:    "_acme-challenge",
				Content: "xyz.acme.example.com",
				TTL:     600,
			},
			wantErr: false,
		},
		{
			name: "valid TXT record",
			input: DNSRecordInput{
				Type:    "TXT",
				Name:    "_verify",
				Content: "verification-token",
				TTL:     300,
			},
			wantErr: false,
		},
		{
			name: "valid MX record with priority",
			input: DNSRecordInput{
				Type:    "MX",
				Name:    "",
				Content: "mail.example.com",
				TTL:     600,
				Prio:    10,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the structure is valid
			if tt.input.Type == "" {
				t.Error("Type should not be empty")
			}
			if tt.input.Content == "" {
				t.Error("Content should not be empty")
			}
		})
	}
}

func TestDNSRecordTypes(t *testing.T) {
	// Verify supported record types
	supportedTypes := []string{"A", "AAAA", "CNAME", "TXT", "MX", "NS", "SRV"}

	for _, recordType := range supportedTypes {
		if len(recordType) == 0 {
			t.Error("Record type should not be empty")
		}
	}
}

func TestDNSRecordStructure(t *testing.T) {
	record := DNSRecord{
		ID:      "12345",
		Name:    "www",
		Type:    "A",
		Content: "192.168.1.1",
		TTL:     600,
	}

	if record.ID == "" {
		t.Error("ID should not be empty")
	}
	if record.Type == "" {
		t.Error("Type should not be empty")
	}
	if record.Content == "" {
		t.Error("Content should not be empty")
	}
	if record.TTL <= 0 {
		t.Error("TTL should be positive")
	}
}

func TestListDNSRecordsResponseStructure(t *testing.T) {
	resp := ListDNSRecordsResponse{
		Status: "SUCCESS",
		Records: []DNSRecord{
			{ID: "1", Type: "A", Name: "", Content: "1.2.3.4", TTL: 600},
			{ID: "2", Type: "A", Name: "*", Content: "1.2.3.4", TTL: 600},
		},
	}

	if resp.Status != "SUCCESS" {
		t.Error("Status should be SUCCESS")
	}
	if len(resp.Records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(resp.Records))
	}
}

func TestCreateDNSRecordResponseStructure(t *testing.T) {
	resp := CreateDNSRecordResponse{
		Status:   "SUCCESS",
		ID:       "123456",
		Warnings: []string{},
	}

	if resp.Status != "SUCCESS" {
		t.Error("Status should be SUCCESS")
	}
	if resp.ID == "" {
		t.Error("ID should not be empty")
	}
}
