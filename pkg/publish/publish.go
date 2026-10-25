// references:
// https://github.com/npm/cli/blob/latest/workspaces/libnpmpublish/lib/provenance.js
// https://github.com/npm/cli/blob/latest/workspaces/libnpmpublish/lib/publish.js

package publish

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	slsav02 "github.com/in-toto/attestation/go/predicates/provenance/v02"
	slsav1 "github.com/in-toto/attestation/go/predicates/provenance/v1"
	intoto "github.com/in-toto/attestation/go/v1"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"github.com/startracex/ciinfo"
	"github.com/startracex/wangpkg/pkg/manifest"
	"github.com/startracex/wangpkg/pkg/npmini"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	streamContentType   = "application/octet-stream"
	sigstoreContentType = "application/vnd.dev.sigstore.bundle+json;version=0.2"

	inTotoTypeV01 = "https://in-toto.io/Statement/v0.1"
	inTotoTypeV1  = "https://in-toto.io/Statement/v1"

	slsaV02 = "https://slsa.dev/provenance/v0.2"
	slsaV1  = "https://slsa.dev/provenance/v1"

	githubBuildType = "https://slsa-framework.github.io/github-actions-buildtypes/workflow/v1"
	gitlabBuildType = "https://github.com/npm/cli/gitlab/v0alpha1"

	githubBuilderIDPrefix = "https://github.com/actions/runner"
)

type Attachment struct {
	ContentType string `json:"content_type"`
	Data        string `json:"data"`
	Length      int64  `json:"length"`
}

type PublishBody struct {
	ID          string                `json:"_id"`
	Name        string                `json:"name"`
	Access      string                `json:"access"`
	Description string                `json:"description"`
	DistTags    map[string]string     `json:"dist-tags"`
	Versions    map[string]any        `json:"versions"`
	Attachments map[string]Attachment `json:"_attachments"`
}

type Options struct {
	*manifest.Manifest
	NpmINI     *npmini.NpmINI
	Transport  http.RoundTripper
	Registry   *url.URL
	CI         ciinfo.Info
	File       io.Reader
	Access     string
	Tag        string
	Provenance bool
}

func Publish(opts Options) (*protobundle.Bundle, error) {
	if !manifest.IsDistTag(opts.Tag) {
		return nil, fmt.Errorf("invalid dist tag: %s", opts.Tag)
	}
	if _, err := semver.StrictNewVersion(opts.Version); err != nil {
		return nil, fmt.Errorf("invalid version %s", opts.Version)
	}
	access, err := opts.getVisibility()
	if err != nil {
		return nil, err
	}
	sr, err := sumReader(opts.File)
	if err != nil {
		return nil, err
	}
	tarballName := opts.Name + "-" + opts.Version + ".tgz"
	provenanceBundleName := opts.Name + "-" + opts.Version + ".sigstore"

	tarballURL := opts.Registry.JoinPath(opts.Name + "/-/" + tarballName)
	attachments := map[string]Attachment{
		tarballName: {
			ContentType: streamContentType,
			Data:        sr.Base64,
			Length:      sr.Size,
		},
	}
	dist := manifest.Dist{
		Integrity: sr.Integrity,
		Shasum:    sr.SHA1,
		Tarball:   tarballURL.String(),
	}

	var bundle *protobundle.Bundle
	if opts.Provenance {
		bundle, err = opts.createProvenance(sr)
		if err != nil {
			return nil, err
		}
		attURL := opts.Registry.JoinPath(
			"/-/npm/v1/attestations/", opts.Name+"@"+opts.Version,
		)

		bunJSON, err := protojson.Marshal(bundle)
		if err != nil {
			return nil, fmt.Errorf("marshal sigstore bundle: %w", err)
		}
		var bundleObject any
		if err := json.Unmarshal(bunJSON, &bundleObject); err != nil {
			return nil, fmt.Errorf("decode sigstore bundle: %w", err)
		}

		dist.Attestations = map[string]any{
			"url": attURL.String(),
			"provenance": map[string]any{
				"sigstoreBundle": bundleObject,
				"predicateType":  slsaV1,
			},
		}
		attachments[provenanceBundleName] = Attachment{
			ContentType: sigstoreContentType,
			Data:        string(bunJSON),
			Length:      int64(len(bunJSON)),
		}
	}

	id := opts.Name + "@" + opts.Version
	obj := opts.Object.Clone()
	obj.Set("_id", id)
	obj.Set("dist", dist)

	body := PublishBody{
		ID:          opts.Name,
		Name:        opts.Name,
		Description: opts.Description,
		Access:      access,
		DistTags:    map[string]string{opts.Tag: opts.Version},
		Versions:    map[string]any{opts.Version: obj},
		Attachments: attachments,
	}

	return bundle, opts.doPublish(body)
}

func (opts Options) getVisibility() (string, error) {
	access := opts.Access
	switch access {
	case "":
		// do nothing
	case "public", "restricted":
		return access, nil
	default:
		return "", fmt.Errorf("invalid access: %s", access)
	}
	isPublished, err := Visibility(opts.NpmINI, opts.Transport, opts.Name)
	if err != nil {
		return "", err
	}
	if isPublished {
		return "public", nil
	}
	return "restricted", nil
}

func (opts Options) createProvenance(sr sumResult) (*protobundle.Bundle, error) {
	startedOn := time.Now().UTC()

	if err := opts.ensureProvenanceGeneration(); err != nil {
		return nil, err
	}

	oidcToken, err := opts.getOIDCToken()
	if err != nil {
		return nil, err
	}

	statement, err := opts.buildProvenanceStatement(sr, startedOn)
	if err != nil {
		return nil, err
	}
	statementJSON, err := (protojson.MarshalOptions{EmitUnpopulated: true}).Marshal(statement)
	if err != nil {
		return nil, fmt.Errorf("marshal in-toto statement: %w", err)
	}

	return SignDSSE(statementJSON, SignOptions{
		IdentityToken: oidcToken,
		Transport:     opts.Transport,
	})
}

func (opts Options) ensureProvenanceGeneration() error {
	env := opts.CI.Env
	if _, exists := env["GITHUB_ACTIONS"]; exists {
		if reqURL := env["ACTIONS_ID_TOKEN_REQUEST_URL"]; reqURL == "" {
			return fmt.Errorf("provenance generation in GitHub Actions requires \"write\" access to the \"id-token\" permission")
		}
		return nil
	}

	if _, exists := os.LookupEnv("GITLAB_CI"); exists {
		if token := env["SIGSTORE_ID_TOKEN"]; token == "" {
			return fmt.Errorf("provenance generation in GitLab CI requires \"SIGSTORE_ID_TOKEN\" with \"sigstore\" audience to be present in \"id_tokens\"")
		}
		return nil
	}

	return fmt.Errorf("automatic provenance generation not supported for this CI provider")
}

func (opts Options) buildProvenanceStatement(sr sumResult, startedOn time.Time) (*intoto.Statement, error) {
	finishedOn := time.Now().UTC()
	env := opts.CI.Env
	subject := []*intoto.ResourceDescriptor{{
		Name:   pkgurl(opts.Name, opts.Version),
		Digest: map[string]string{"sha512": sr.SHA512},
	}}

	if opts.CI.Vendors["GITHUB_ACTIONS"] {
		workflowPath, workflowRef, _ := strings.Cut(strings.TrimPrefix(env["GITHUB_WORKFLOW_REF"], env["GITHUB_REPOSITORY"]+"/"), "@")
		invocationID := env["GITHUB_SERVER_URL"] + "/" + env["GITHUB_REPOSITORY"] + "/actions/runs/" + env["GITHUB_RUN_ID"] + "/attempts/" + env["GITHUB_RUN_ATTEMPT"]

		external, err := newStruct(map[string]any{
			"workflow": map[string]string{
				"ref":        workflowRef,
				"repository": env["GITHUB_SERVER_URL"] + "/" + env["GITHUB_REPOSITORY"],
				"path":       workflowPath,
			},
		})
		if err != nil {
			return nil, err
		}
		internal, err := newStruct(map[string]any{
			"github": map[string]string{
				"event_name":          env["GITHUB_EVENT_NAME"],
				"repository_id":       env["GITHUB_REPOSITORY_ID"],
				"repository_owner_id": env["GITHUB_REPOSITORY_OWNER_ID"],
			},
		})
		if err != nil {
			return nil, err
		}

		predicate, err := newStruct(&slsav1.Provenance{
			BuildDefinition: &slsav1.BuildDefinition{
				BuildType:          githubBuildType,
				ExternalParameters: external,
				InternalParameters: internal,
				ResolvedDependencies: []*intoto.ResourceDescriptor{{
					Uri:    "git+" + env["GITHUB_SERVER_URL"] + "/" + env["GITHUB_REPOSITORY"] + "@" + env["GITHUB_REF"],
					Digest: map[string]string{"gitCommit": env["GITHUB_SHA"]},
				}},
			},
			RunDetails: &slsav1.RunDetails{
				Builder: &slsav1.Builder{
					Id: githubBuilderIDPrefix + "/" + env["RUNNER_ENVIRONMENT"],
				},
				Metadata: &slsav1.BuildMetadata{
					InvocationId: invocationID,
					StartedOn:    timestamppb.New(startedOn),
					FinishedOn:   timestamppb.New(finishedOn),
				},
			},
		})
		if err != nil {
			return nil, err
		}

		return &intoto.Statement{
			Type:          inTotoTypeV1,
			Subject:       subject,
			PredicateType: slsaV1,
			Predicate:     predicate,
		}, nil
	}

	if opts.CI.Vendors["GITLAB"] {
		parameters, err := newStruct(pickKeys(env, gitlabEnvKeys))
		if err != nil {
			return nil, err
		}
		environment, err := newStruct(map[string]any{
			"name":         env["CI_RUNNER_DESCRIPTION"],
			"architecture": env["CI_RUNNER_EXECUTABLE_ARCH"],
			"server":       env["CI_SERVER_URL"],
			"project":      env["CI_PROJECT_PATH"],
			"job":          map[string]string{"id": env["CI_JOB_ID"]},
			"pipeline":     map[string]string{"id": env["CI_PIPELINE_ID"], "ref": env["CI_CONFIG_PATH"]},
		})
		if err != nil {
			return nil, err
		}

		predicate, err := newStruct(&slsav02.Provenance{
			BuildType: gitlabBuildType,
			Builder: &slsav02.Builder{
				Id: env["CI_PROJECT_URL"] + "/-/runners/" + env["CI_RUNNER_ID"],
			},
			Invocation: &slsav02.Invocation{
				ConfigSource: &slsav02.ConfigSource{
					Uri:        "git+" + env["CI_PROJECT_URL"],
					Digest:     map[string]string{"sha1": env["CI_COMMIT_SHA"]},
					EntryPoint: env["CI_JOB_NAME"],
				},
				Parameters:  parameters,
				Environment: environment,
			},
			Metadata: &slsav02.Metadata{
				BuildInvocationId: env["CI_JOB_URL"],
				Completeness: &slsav02.Completeness{
					Parameters:  true,
					Environment: true,
					Materials:   false,
				},
				Reproducible: false,
			},
			Materials: []*slsav02.Material{{
				Uri:    "git+" + env["CI_PROJECT_URL"],
				Digest: map[string]string{"sha1": env["CI_COMMIT_SHA"]},
			}},
		})
		if err != nil {
			return nil, err
		}

		return &intoto.Statement{
			Type:          inTotoTypeV01,
			Subject:       subject,
			PredicateType: slsaV02,
			Predicate:     predicate,
		}, nil
	}

	return nil, fmt.Errorf("unsupported CI for provenance generation")
}

func (opts Options) doPublish(body PublishBody) error {
	out := new(bytes.Buffer)
	if err := json.MarshalWrite(out, body); err != nil {
		return err
	}
	return PublishContent(opts.NpmINI, opts.Transport, opts.Name, out)
}

func (opts Options) getOIDCToken() (string, error) {
	env := opts.CI.Env
	// Generic sigstore token
	if t := env["SIGSTORE_ID_TOKEN"]; t != "" {
		return t, nil
	}

	// GitHub Actions OIDC. The token endpoint only accepts GET (see GitHub's
	// docs and npm's provenance implementation): a POST is rejected.
	reqURL := env["ACTIONS_ID_TOKEN_REQUEST_URL"]
	reqToken := env["ACTIONS_ID_TOKEN_REQUEST_TOKEN"]
	if reqURL != "" && reqToken != "" {
		u, err := url.Parse(reqURL)
		if err != nil {
			return "", fmt.Errorf("parse OIDC request URL: %w", err)
		}
		q := u.Query()
		q.Add("audience", "sigstore")
		u.RawQuery = q.Encode()

		req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
		req.Header.Set("Authorization", "Bearer "+reqToken)
		req.Header.Set("Accept", "application/json")
		resp, err := opts.Transport.RoundTrip(req)
		if err != nil {
			return "", fmt.Errorf("request GitHub OIDC token: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return "", fmt.Errorf("GitHub OIDC token request: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var result struct {
			Value string `json:"value"`
		}
		if err := json.UnmarshalRead(resp.Body, &result); err != nil {
			return "", fmt.Errorf("decode GitHub OIDC token: %w", err)
		}
		if result.Value != "" {
			if err := checkAudience(result.Value, "sigstore"); err != nil {
				return "", err
			}
			return result.Value, nil
		}
	}

	// GitLab CI JWT (needs SIGSTORE_ID_TOKEN with sigstore audience)
	if t := env["SIGSTORE_ID_TOKEN"]; t != "" {
		return t, nil
	}

	return "", fmt.Errorf("OIDC token not found")
}

var gitlabEnvKeys = []string{
	"CI_COMMIT_TITLE",
	"CI_PROJECT_DESCRIPTION",
	"CI_PROJECT_URL",
	"RUNNER_GENERATE_ARTIFACTS_METADATA",
	"CI",
	"CI_BUILD_NAME",
	"CI_BUILD_REF",
	"CI_BUILD_STAGE",
	"CI_COMMIT_BEFORE_SHA",
	"CI_DEPENDENCY_PROXY_GROUP_IMAGE_PREFIX",
	"CI_NODE_TOTAL",
	"CI_SERVER_PROTO",
	"CI_CONFIG_PATH",
	"CI_JOB_NAME",
	"CI_PIPELINE_IID",
	"CI_PIPELINE_URL",
	"CI_REGISTRY_IMAGE",
	"CI_RUNNER_ID",
	"CI_SERVER_SHELL_SSH_PORT",
	"CI_COMMIT_SHA",
	"CI_DEPENDENCY_PROXY_DIRECT_GROUP_IMAGE_PREFIX",
	"CI_PROJECT_CLASSIFICATION_LABEL",
	"CI_PROJECT_NAMESPACE_ID",
	"CI_API_GRAPHQL_URL",
	"CI_DEPENDENCY_PROXY_SERVER",
	"CI_DEPENDENCY_PROXY_USER",
	"CI_JOB_STAGE",
	"CI_PROJECT_PATH",
	"CI_PROJECT_TITLE",
	"CI_SERVER_VERSION_MINOR",
	"CI_API_V4_URL",
	"CI_BUILD_BEFORE_SHA",
	"CI_PIPELINE_CREATED_AT",
	"CI_PROJECT_VISIBILITY",
	"CI_COMMIT_REF_SLUG",
	"CI_DEFAULT_BRANCH",
	"CI_JOB_ID",
	"CI_JOB_STARTED_AT",
	"CI_PROJECT_ID",
	"CI_PROJECT_NAME",
	"CI_REGISTRY_USER",
	"CI_SERVER_HOST",
	"CI_COMMIT_REF_PROTECTED",
	"CI_COMMIT_TIMESTAMP",
	"CI_PIPELINE_ID",
	"CI_PROJECT_NAMESPACE",
	"CI_SERVER_PORT",
	"GITLAB_USER_ID",
	"CI_JOB_URL",
	"CI_PROJECT_REPOSITORY_LANGUAGES",
	"CI_SERVER_NAME",
	"GITLAB_CI",
	"CI_BUILD_REF_NAME",
	"CI_RUNNER_DESCRIPTION",
	"CI_BUILD_REF_SLUG",
	"CI_COMMIT_SHORT_SHA",
	"CI_SERVER_REVISION",
	"GITLAB_USER_LOGIN",
	"CI_COMMIT_BRANCH",
	"CI_PIPELINE_SOURCE",
	"CI_PROJECT_PATH_SLUG",
	"CI_SERVER_VERSION",
	"CI_TEMPLATE_REGISTRY_HOST",
	"CI_PAGES_DOMAIN",
	"CI_SERVER_VERSION_MAJOR",
	"CI_COMMIT_REF_NAME",
	"CI_JOB_NAME_SLUG",
	"CI_PAGES_URL",
	"CI_REGISTRY",
	"CI_SERVER_VERSION_PATCH",
	"GITLAB_FEATURES",
	"CI_BUILD_ID",
	"CI_PROJECT_ROOT_NAMESPACE",
	"CI_RUNNER_TAGS",
	"CI_SERVER_SHELL_SSH_HOST",
	"CI_SERVER_URL",
}
