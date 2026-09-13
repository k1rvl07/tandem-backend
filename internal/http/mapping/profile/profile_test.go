package profile

import (
	"reflect"
	"testing"

	cprofile "github.com/tandem/tandem/internal/contracts/profile"
	dtoprofile "github.com/tandem/tandem/internal/http/dto/profile"
)

func TestToContractUpdateProfile(t *testing.T) {
	got := ToContractUpdateProfile(dtoprofile.UpdateProfileRequest{
		DisplayName: "Ivan",
		Bio:         "Bio",
	})
	if !reflect.DeepEqual(got, cprofile.UpdateProfileRequest{DisplayName: "Ivan", Bio: "Bio"}) {
		t.Fatalf("ToContractUpdateProfile = %+v", got)
	}
}

func TestToContractChangePassword(t *testing.T) {
	got := ToContractChangePassword(dtoprofile.ChangePasswordRequest{
		NewPassword:     "new",
		CurrentPassword: "old",
	})
	if !reflect.DeepEqual(got, cprofile.ChangePasswordRequest{NewPassword: "new", CurrentPassword: "old"}) {
		t.Fatalf("ToContractChangePassword = %+v", got)
	}
}
