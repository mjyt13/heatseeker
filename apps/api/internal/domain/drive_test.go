package domain

import "testing"

func TestCanPublishToDrive(t *testing.T) {
	shared := "0AShared"
	failed := "revoked"
	myDrive := &DriveConnection{Writable: true}
	readOnly := &DriveConnection{}
	sharedDrive := &DriveConnection{Writable: true, DriveID: &shared}
	ok := &DrivePublisher{Email: "head@example.com"}
	revoked := &DrivePublisher{Email: "head@example.com", LastError: &failed}
	tests := []struct {
		name string
		conn *DriveConnection
		pub  *DrivePublisher
		want bool
	}{
		{"no folder", nil, ok, false},
		{"my drive, service account only", myDrive, nil, false},
		{"my drive, publisher", myDrive, ok, true},
		{"read-only share, publisher", readOnly, ok, true},
		{"my drive, revoked publisher", myDrive, revoked, false},
		{"shared drive, service account", sharedDrive, nil, true},
		{"shared drive, revoked publisher falls back", sharedDrive, revoked, true},
		{"read-only shared drive", &DriveConnection{DriveID: &shared}, nil, false},
	}
	for _, tt := range tests {
		if got := CanPublishToDrive(tt.conn, tt.pub); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}
