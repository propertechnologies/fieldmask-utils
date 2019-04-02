package fieldmask_utils_test

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/propertechnologies/fieldmask-utils"
	"github.com/propertechnologies/fieldmask-utils/testproto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testUserFull *testproto.User
var testUserPartial *testproto.User

func init() {
	ts := &timestamp.Timestamp{
		Seconds: 5, // easy to verify
		Nanos:   6, // easy to verify
	}
	serializedTs, _ := proto.Marshal(ts)

	friend1 := &testproto.User{
		Id:          2,
		Username:    "friend",
		Role:        testproto.Role_REGULAR,
		Meta:        map[string]string{"foo": "bar"},
		Deactivated: true,
		Permissions: []testproto.Permission{testproto.Permission_READ, testproto.Permission_WRITE},
		Avatar: &testproto.Image{
			OriginalUrl: "original.jpg",
			ResizedUrl:  "resized.jpg",
		},
		Images: []*testproto.Image{
			{
				OriginalUrl: "FRIEND original_image1.jpg",
				ResizedUrl:  "FRIEND resized_image1.jpg",
			},
			{
				OriginalUrl: "FRIEND original_image2.jpg",
				ResizedUrl:  "FRIEND resized_image2.jpg",
			},
		},
		Tags: []string{"FRIEND tag1", "FRIEND tag2", "FRIEND tag3"},
		Name: &testproto.User_FemaleName{FemaleName: "Maggy"},
	}
	testUserFull = &testproto.User{
		Id:          1,
		Username:    "username",
		Role:        testproto.Role_ADMIN,
		Meta:        map[string]string{"foo": "bar"},
		Deactivated: true,
		Permissions: []testproto.Permission{testproto.Permission_READ, testproto.Permission_WRITE},
		Avatar: &testproto.Image{
			OriginalUrl: "original.jpg",
			ResizedUrl:  "resized.jpg",
		},
		Images: []*testproto.Image{
			{
				OriginalUrl: "original_image1.jpg",
				ResizedUrl:  "resized_image1.jpg",
			},
			{
				OriginalUrl: "original_image2.jpg",
				ResizedUrl:  "resized_image2.jpg",
			},
		},
		Tags:    []string{"tag1", "tag2", "tag3"},
		Friends: []*testproto.User{friend1},
		Name:    &testproto.User_MaleName{MaleName: "John"},
		Details: []*any.Any{
			{
				TypeUrl: "example.com/example/" + proto.MessageName(ts),
				Value:   serializedTs,
			},
		},
	}
	testUserPartial = &testproto.User{
		Id:       1,
		Username: "username",
	}
}

func TestStructToStructProtoSuccess(t *testing.T) {
	userDst := &testproto.User{}
	mask := fieldmask_utils.MaskFromString(
		"Id,Avatar{OriginalUrl},Tags,Images,Permissions,Friends{Images{ResizedUrl}},Name{MaleName}")
	err := fieldmask_utils.StructToStruct(mask, testUserFull, userDst)
	require.NoError(t, err)
	assert.Equal(t, testUserFull.Id, userDst.Id)
	assert.Equal(t, testUserFull.Avatar.OriginalUrl, userDst.Avatar.OriginalUrl)
	assert.Equal(t, "", userDst.Avatar.ResizedUrl)
	assert.Equal(t, testUserFull.Tags, userDst.Tags)
	assert.Equal(t, testUserFull.Images, userDst.Images)
	assert.Equal(t, testUserFull.Name, userDst.Name)
	assert.Equal(t, testUserFull.Permissions, userDst.Permissions)
	assert.Equal(t, len(testUserFull.Friends), len(userDst.Friends))
	assert.Equal(t, len(testUserFull.Friends[0].Images), len(userDst.Friends[0].Images))
	assert.Equal(t, testUserFull.Friends[0].Images[0].ResizedUrl, userDst.Friends[0].Images[0].ResizedUrl)
	assert.Equal(t, "", userDst.Friends[0].Images[0].OriginalUrl)
	// Zero (default) values below.
	assert.Equal(t, testproto.Role_UNKNOWN, userDst.Role)
	assert.Equal(t, false, userDst.Deactivated)
	assert.Equal(t, map[string]string(nil), userDst.Meta)
}

func TestStructToStructEmptyMaskSuccess(t *testing.T) {
	userDst := &testproto.User{}
	mask := fieldmask_utils.MaskFromString("")
	err := fieldmask_utils.StructToStruct(mask, testUserFull, userDst)
	require.NoError(t, err)
	assert.Equal(t, testUserFull, userDst)
}

func TestStructToStructPartialProtoSuccess(t *testing.T) {
	userDst := &testproto.User{}
	mask := fieldmask_utils.MaskFromString(
		"Id,Avatar{OriginalUrl},Images,Username,Permissions,Name{MaleName}")
	err := fieldmask_utils.StructToStruct(mask, testUserPartial, userDst)
	assert.Nil(t, err)
	assert.Equal(t, testUserPartial.Id, userDst.Id)
	assert.Equal(t, testUserPartial.Username, userDst.Username)
	assert.Equal(t, testUserPartial.Name, userDst.Name)
}

func TestStructToStructMaskInverse(t *testing.T) {
	userSrc := &testproto.User{
		Id:          1,
		Username:    "username",
		Role:        testproto.Role_ADMIN,
		Meta:        map[string]string{"foo": "bar"},
		Deactivated: false,
		Permissions: []testproto.Permission{testproto.Permission_EXECUTE},
		Name:        &testproto.User_FemaleName{FemaleName: "Dana"},
		Friends: []*testproto.User{
			{Id: 2, Username: "friend1"},
			{Id: 3, Username: "friend2"},
		},
	}
	userDst := &testproto.User{}
	mask := fieldmask_utils.MaskInverse{"Id": nil, "Friends": fieldmask_utils.MaskInverse{"Username": nil}}
	err := fieldmask_utils.StructToStruct(mask, userSrc, userDst)
	require.NoError(t, err)
	// Verify that Id is not copied.
	assert.Equal(t, uint32(0), userDst.Id)
	// Verify that Friend Usernames are not copied.
	assert.Equal(t, "", userDst.Friends[0].Username)
	assert.Equal(t, "", userDst.Friends[1].Username)
	// Copy missed fields manually and then compare these structs.
	userDst.Id = userSrc.Id
	userDst.Friends[0].Username = userSrc.Friends[0].Username
	userDst.Friends[1].Username = userSrc.Friends[1].Username
	assert.Equal(t, userSrc, userDst)
}

type Name interface {
	someMethod()
}

type FemaleName struct {
	FemaleName string
}

func (*FemaleName) someMethod() {}
func (f *FemaleName) String() string {
	return f.FemaleName
}

type CustomUser struct {
	Name Name
}

func TestStructToStructProtoDifferentInterfacesFail(t *testing.T) {
	userDst := &testproto.User{}
	userSrc := &CustomUser{Name: &FemaleName{FemaleName: "Dana"}}

	mask := fieldmask_utils.MaskFromString("Name")
	err := fieldmask_utils.StructToStruct(mask, userSrc, userDst)
	assert.NotNil(t, err)
}

func TestStructToStructProtoSameInterfacesSuccess(t *testing.T) {
	type User1 struct {
		Stringer fmt.Stringer
	}

	type User2 struct {
		Stringer fmt.Stringer
	}

	src := &User1{
		Stringer: &FemaleName{FemaleName: "Jessica"},
	}

	dst := &User2{}

	mask := fieldmask_utils.MaskFromString("Stringer")
	err := fieldmask_utils.StructToStruct(mask, src, dst)
	assert.Nil(t, err)
	assert.Equal(t, src.Stringer.String(), dst.Stringer.String())
}

func TestStructToStructNonProtoSuccess(t *testing.T) {
	type Image struct {
		OriginalUrl string
		ResizedUrl  string
	}
	type User struct {
		Id          uint32
		Username    string
		Deactivated bool
		Images      []*Image
	}

	userSrc := &User{
		Id:          1,
		Username:    "johnny",
		Deactivated: true,
		Images: []*Image{
			{"original_url.jpg", "resized_url.jpg"},
			{"original_url.jpg", "resized_url.jpg"},
		},
	}
	userDst := &testproto.User{}
	mask := fieldmask_utils.MaskFromString("")
	err := fieldmask_utils.StructToStruct(mask, userSrc, userDst)
	assert.Nil(t, err)
	assert.Equal(t, userSrc.Id, userDst.Id)
	assert.Equal(t, userSrc.Username, userDst.Username)
	assert.Equal(t, len(userSrc.Images), len(userDst.Images))
	assert.Equal(t, userSrc.Images[0].OriginalUrl, userDst.Images[0].OriginalUrl)
	assert.Equal(t, userSrc.Images[0].ResizedUrl, userDst.Images[0].ResizedUrl)
	assert.Equal(t, userSrc.Images[1].OriginalUrl, userDst.Images[1].OriginalUrl)
	assert.Equal(t, userSrc.Images[1].ResizedUrl, userDst.Images[1].ResizedUrl)
	assert.Equal(t, userSrc.Deactivated, userDst.Deactivated)
}

func TestStructToStructNonProtoFail(t *testing.T) {
	type User struct {
		Id           uint32
		UnknownField string
		Deactivated  bool
	}

	userSrc := &User{
		Id:           1,
		UnknownField: "johnny",
		Deactivated:  true,
	}
	userDst := &testproto.User{}
	mask := fieldmask_utils.MaskFromString("")
	err := fieldmask_utils.StructToStruct(mask, userSrc, userDst)
	assert.NotNil(t, err)
}

func TestStructToMapSuccess(t *testing.T) {
	userDst := make(map[string]interface{})
	mask := fieldmask_utils.MaskFromString(
		"id,avatar{original_url},tags,images,permissions,friends{images{resized_url}}")
	err := fieldmask_utils.StructToMap(mask, testUserFull, userDst)
	assert.Nil(t, err)
	expected := map[string]interface{}{
		"id": testUserFull.Id,
		"avatar": map[string]interface{}{
			"original_url": testUserFull.Avatar.OriginalUrl,
		},
		"tags": testUserFull.Tags,
		"images": []map[string]interface{}{
			{"original_url": testUserFull.Images[0].OriginalUrl, "resized_url": testUserFull.Images[0].ResizedUrl},
			{"original_url": testUserFull.Images[1].OriginalUrl, "resized_url": testUserFull.Images[1].ResizedUrl},
		},
		"permissions": testUserFull.Permissions,
		"friends": []map[string]interface{}{
			{
				"images": []map[string]interface{}{
					{"resized_url": testUserFull.Friends[0].Images[0].ResizedUrl},
					{"resized_url": testUserFull.Friends[0].Images[1].ResizedUrl},
				},
			},
		},
	}
	assert.Equal(t, expected, userDst)
}

func TestStructToMapPartialProtoSuccess(t *testing.T) {
	userDst := make(map[string]interface{})
	mask := fieldmask_utils.MaskFromString(
		"id,avatar{original_url},images,username,permissions,name{male_name}")
	err := fieldmask_utils.StructToMap(mask, testUserPartial, userDst)
	assert.Nil(t, err)
	expected := map[string]interface{}{
		"id":          testUserPartial.Id,
		"avatar":      nil,
		"images":      []map[string]interface{}{},
		"username":    testUserPartial.Username,
		"permissions": []interface{}(nil),
	}
	assert.Equal(t, expected, userDst)
}
