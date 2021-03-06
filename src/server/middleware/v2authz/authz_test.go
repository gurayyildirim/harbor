// Copyright Project Harbor Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package authz

import (
	"github.com/goharbor/harbor/src/common/models"
	"github.com/goharbor/harbor/src/common/rbac"
	"github.com/goharbor/harbor/src/core/filter"
	"github.com/goharbor/harbor/src/core/promgr/metamgr"
	"github.com/goharbor/harbor/src/server/middleware"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

type mockPM struct{}

func (mockPM) Get(projectIDOrName interface{}) (*models.Project, error) {
	name := projectIDOrName.(string)
	id, _ := strconv.Atoi(strings.TrimPrefix(name, "project_"))
	if id == 0 {
		return nil, nil
	}
	return &models.Project{
		ProjectID: int64(id),
		Name:      name,
	}, nil
}

func (mockPM) Create(*models.Project) (int64, error) {
	panic("implement me")
}

func (mockPM) Delete(projectIDOrName interface{}) error {
	panic("implement me")
}

func (mockPM) Update(projectIDOrName interface{}, project *models.Project) error {
	panic("implement me")
}

func (mockPM) List(query *models.ProjectQueryParam) (*models.ProjectQueryResult, error) {
	panic("implement me")
}

func (mockPM) IsPublic(projectIDOrName interface{}) (bool, error) {
	return false, nil
}

func (mockPM) Exists(projectIDOrName interface{}) (bool, error) {
	panic("implement me")
}

func (mockPM) GetPublic() ([]*models.Project, error) {
	panic("implement me")
}

func (mockPM) GetMetadataManager() metamgr.ProjectMetadataManager {
	panic("implement me")
}

type mockSC struct{}

func (mockSC) IsAuthenticated() bool {
	return true
}

func (mockSC) GetUsername() string {
	return "mock"
}

func (mockSC) IsSysAdmin() bool {
	return false
}

func (mockSC) IsSolutionUser() bool {
	return false
}

func (mockSC) GetMyProjects() ([]*models.Project, error) {
	panic("implement me")
}

func (mockSC) GetProjectRoles(projectIDOrName interface{}) []int {
	panic("implement me")
}

func (mockSC) Can(action rbac.Action, resource rbac.Resource) bool {
	ns, _ := resource.GetNamespace()
	perms := map[int64]map[rbac.Action]struct{}{
		1: {
			rbac.ActionPull: {},
			rbac.ActionPush: {},
		},
		2: {
			rbac.ActionPull: {},
		},
	}
	pid := ns.Identity().(int64)
	m, ok := perms[pid]
	if !ok {
		return false
	}
	_, ok = m[action]
	return ok
}

func TestMain(m *testing.M) {
	checker = reqChecker{
		pm: mockPM{},
	}
	if rc := m.Run(); rc != 0 {
		os.Exit(rc)
	}
}

func TestMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	baseCtx := context.WithValue(context.Background(), filter.SecurCtxKey, mockSC{})
	ar1 := &middleware.ArtifactInfo{
		Repository:  "project_1/hello-world",
		Reference:   "v1",
		ProjectName: "project_1",
	}
	ar2 := &middleware.ArtifactInfo{
		Repository:  "library/ubuntu",
		Reference:   "14.04",
		ProjectName: "library",
	}
	ar3 := &middleware.ArtifactInfo{
		Repository:           "project_1/ubuntu",
		Reference:            "14.04",
		ProjectName:          "project_1",
		BlobMountRepository:  "project_2/ubuntu",
		BlobMountProjectName: "project_2",
		BlobMountDigest:      "sha256:08e4a417ff4e3913d8723a05cc34055db01c2fd165b588e049c5bad16ce6094f",
	}
	ar4 := &middleware.ArtifactInfo{
		Repository:           "project_1/ubuntu",
		Reference:            "14.04",
		ProjectName:          "project_1",
		BlobMountRepository:  "project_3/ubuntu",
		BlobMountProjectName: "project_3",
		BlobMountDigest:      "sha256:08e4a417ff4e3913d8723a05cc34055db01c2fd165b588e049c5bad16ce6094f",
	}
	ctx1 := context.WithValue(baseCtx, middleware.ArtifactInfoKey, ar1)
	ctx2 := context.WithValue(baseCtx, middleware.ArtifactInfoKey, ar2)
	ctx3 := context.WithValue(baseCtx, middleware.ArtifactInfoKey, ar3)
	ctx4 := context.WithValue(baseCtx, middleware.ArtifactInfoKey, ar4)
	req1a, _ := http.NewRequest(http.MethodGet, "/v2/project_1/hello-world/manifest/v1", nil)
	req1b, _ := http.NewRequest(http.MethodDelete, "/v2/project_1/hello-world/manifest/v1", nil)
	req2, _ := http.NewRequest(http.MethodGet, "/v2/library/ubuntu/manifest/14.04", nil)
	req3, _ := http.NewRequest(http.MethodGet, "/v2/_catalog", nil)
	req4, _ := http.NewRequest(http.MethodPost, "/v2/project_1/ubuntu/blobs/uploads/mount=?mount=sha256:08e4a417ff4e3913d8723a05cc34055db01c2fd165b588e049c5bad16ce6094f&from=project_2/ubuntu", nil)
	req5, _ := http.NewRequest(http.MethodPost, "/v2/project_1/ubuntu/blobs/uploads/mount=?mount=sha256:08e4a417ff4e3913d8723a05cc34055db01c2fd165b588e049c5bad16ce6094f&from=project_3/ubuntu", nil)

	cases := []struct {
		input  *http.Request
		status int
	}{
		{
			input:  req1a.WithContext(ctx1),
			status: http.StatusOK,
		},
		{
			input:  req1b.WithContext(ctx1),
			status: http.StatusOK,
		},
		{
			input:  req2.WithContext(ctx2),
			status: http.StatusUnauthorized,
		},
		{
			input:  req3.WithContext(baseCtx),
			status: http.StatusUnauthorized,
		},
		{
			input:  req4.WithContext(ctx3),
			status: http.StatusOK,
		},
		{
			input:  req5.WithContext(ctx4),
			status: http.StatusUnauthorized,
		},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		t.Logf("req : %s, %s", c.input.Method, c.input.URL)
		Middleware()(next).ServeHTTP(rec, c.input)
		assert.Equal(t, c.status, rec.Result().StatusCode)
	}
}
