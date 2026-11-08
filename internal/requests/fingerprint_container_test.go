//go:build container

package requests

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func TestPinnedNativeFingerprintTask(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	folder := t.TempDir()
	if err := os.Chmod(folder, 0755); err != nil {
		t.Fatal(err)
	}
	command := func(name string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = folder
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", name, err, out)
		}
	}
	build := exec.CommandContext(ctx, "go", "build", "-o", filepath.Join(folder, "controller"), "../../cmd/request-controller")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0", "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}
	c, selection := setup(t)
	r, err := c.Submit(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	artifact := "inventory.md"
	profile := engineProfile(r, "/proof", false, artifact)
	data, _ := json.Marshal(profile)
	write := func(name string, data []byte) {
		t.Helper()
		p := filepath.Join(folder, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("profile.json", data)
	write("source/"+r.Dependency.PackageFile, []byte("image: nginx:1.25.3\n"))
	write("source/"+artifact, []byte("# Inventory\n- Source digest: `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`\n"))
	command("git", "-C", "source", "init", "-q")
	command("git", "-C", "source", "add", ".")
	command("git", "-C", "source", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	command("git", "clone", "--bare", "source", "source.git")
	command("git", "clone", "source.git", "repo")
	write("proof.mjs", []byte(`
import assert from 'node:assert/strict';
import {readFile,access} from 'node:fs/promises';
import {GlobalConfig} from '/usr/local/renovate/dist/config/global.js';
import {initRepo,getRepoStatus} from '/usr/local/renovate/dist/util/git/index.js';
import execute from '/usr/local/renovate/dist/workers/repository/update/branch/execute-post-upgrade-commands.js';
const profile=JSON.parse(await readFile('/proof/profile.json','utf8'));
const tasks=profile.force.postUpgradeTasks;
GlobalConfig.set({...profile,localDir:'/proof/repo',cacheDir:'/tmp/cache',allowShellExecutorForPostUpgradeCommands:false,binarySource:'global'});
await initRepo({currentBranch:'master',url:'/proof/source.git'});await getRepoStatus();
const result=await execute({manager:'kubernetes',branchName:'fixture',updatedPackageFiles:[{type:'addition',path:'k8s/test.yaml',contents:'image: nginx:1.26.3\n'}],updatedArtifacts:[],upgrades:[{depName:'nginx',postUpgradeTasks:tasks}],postUpgradeTasks:tasks});
assert.deepEqual(result.artifactErrors,[]);
assert.deepEqual(result.updatedArtifacts.map(x=>x.path),['inventory.md']);
const before=await readFile('/proof/source/inventory.md','utf8');
const after=await readFile('/proof/repo/inventory.md','utf8');
assert.notEqual(before,after);
assert.equal(before.replace(/[a-f0-9]{64}/,'digest'),after.replace(/[a-f0-9]{64}/,'digest'));
const unsafe={...tasks,commands:[tasks.commands[0]+'; touch /tmp/unsafe-task']};
const denied=await execute({updatedPackageFiles:[{type:'addition',path:'k8s/test.yaml',contents:'image: nginx:1.26.3\n'}],updatedArtifacts:[],upgrades:[],postUpgradeTasks:unsafe});
assert.ok(denied.artifactErrors.length>0);
await assert.rejects(access('/tmp/unsafe-task'));
`))
	command("docker", "run", "--rm", "--network", "none", "--user", strconv.Itoa(os.Getuid())+":"+strconv.Itoa(os.Getgid()), "-e", "HOME=/tmp", "--entrypoint", "/usr/local/renovate/node", "-v", folder+":/proof", "-v", filepath.Join(folder, "controller")+":/usr/local/bin/image-upgrades-controller:ro", snapshot.RenovateImage, "/proof/proof.mjs")
}
