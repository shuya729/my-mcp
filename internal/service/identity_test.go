package service

import (
	"reflect"
	"testing"
)

func TestIdentityServiceCurrent(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		want   Identity
	}{
		{name: "user ID", userID: "user-123", want: Identity{UserID: "user-123"}},
		{name: "empty user ID", want: Identity{}},
	}

	service := IdentityService{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := service.Current(test.userID)
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("Current(%q) = %v, want %v", test.userID, got, test.want)
			}
		})
	}
}
