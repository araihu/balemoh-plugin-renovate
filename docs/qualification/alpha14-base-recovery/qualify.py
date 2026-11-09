#!/usr/bin/env python3
"""Exercise the candidate binaries in the private GitHub fixture, never production."""
import base64
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import signal
import ssl
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import uuid

REPOSITORY = 'guilycst/balemoh-renovate-engine-poc-20261101'
BINARY = '/usr/local/bin/image-upgrades-controller'
BACKUP = '/usr/local/bin/image-upgrades-request-backup'
ARTIFACT = 'qualification/alpha14-inventory.md'
OLD_DIGEST = 'sha256:c7a6ad68be85142c7fe1089e48faa1e7c7166a194caa9180ddea66345876b9d2'
NEW_DIGEST = 'sha256:41b194461e4bae16f9b25d68b0976ed4735b89ca625c89aad88e1c1c3b7e8860'
OLD = 'docker.io/library/nginx:1.25.3@' + OLD_DIGEST
NEW = 'docker.io/library/nginx:1.26.3@' + NEW_DIGEST


def encode(value):
    return json.dumps(value, separators=(',', ':'), ensure_ascii=False).encode()


def identity(*values):
    return hashlib.sha256(encode(values)).hexdigest()


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *_args, **_kwargs):
        return None


def run():
    assert os.environ['GITHUB_REPOSITORY'] == REPOSITORY, 'fixture repository required'
    branch = os.environ['GITHUB_REF_NAME']
    assert re.fullmatch(r'qualification/alpha14-[a-z0-9-]+', branch), 'disposable fixture base required'
    source = os.environ['CANDIDATE_SOURCE']
    assert re.fullmatch(r'[a-f0-9]{40}', source)
    token = os.environ['RENOVATE_TOKEN']
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def api(path, body=None, method=None):
        req = urllib.request.Request('https://api.github.com/repos/' + REPOSITORY + path,
            data=None if body is None else encode(body), method=method,
            headers={'Authorization': 'Bearer ' + token, 'Accept': 'application/vnd.github+json',
                     'Content-Type': 'application/json', 'X-GitHub-Api-Version': '2022-11-28'})
        with opener.open(req, timeout=30) as response:
            return json.load(response)

    def ref(name):
        return api('/git/ref/heads/' + name)['object']['sha']

    def inventory():
        rows = api('/pulls?state=all&per_page=100')
        assert len(rows) < 100, 'fixture PR bound exceeded'
        return {p['number']: (p['head']['sha'], p['state'], p['merged_at']) for p in rows}

    main = ref('main')
    previous = inventory()
    old_branches = {r['name']: r['commit']['sha'] for r in api('/branches?per_page=100')}
    assert len(old_branches) < 100
    original_base = ref(branch)
    assert original_base == os.environ['GITHUB_SHA'], 'workflow base moved'

    def commit(expected, files, message):
        assert ref(branch) == expected, 'fixture base changed concurrently'
        parent = api('/git/commits/' + expected)
        tree = api('/git/trees', {'base_tree': parent['tree']['sha'], 'tree': [
            {'path': name, 'mode': '100644', 'type': 'blob', 'content': text}
            for name, text in files.items()]})
        result = api('/git/commits', {'message': message, 'tree': tree['sha'], 'parents': [expected]})
        api('/git/refs/heads/' + branch, {'sha': result['sha'], 'force': False}, 'PATCH')
        return result['sha']

    def artifact(generation):
        return '# Synthetic inventory\n- Source digest: `' + '0' * 64 + '`\nGeneration: ' + str(generation) + '\n'

    def manifest(name):
        return ('apiVersion: v1\nkind: Pod\nmetadata:\n  name: alpha14-' + name + '\nspec:\n  containers:\n'
                '    - name: proxy\n      image: ' + OLD + '\n'
                '    - name: unselected\n      image: docker.io/library/alpine:3.19.0\n')
    files = {f'k8s/alpha14-{name}.yaml': manifest(name) for name in ['queued', 'blocked', 'changed']}
    files[ARTIFACT] = artifact(1)
    files['renovate.json'] = json.dumps({'enabled': True, 'dependencyDashboard': False,
        'packageRules': [{'matchPackageNames': ['*'], 'enabled': False}],
        'prHourlyLimit': 60, 'prConcurrentLimit': 60}) + '\n'
    approved_base = commit(original_base, files, 'test: add isolated alpha.14 fixtures')
    base = commit(approved_base, {'qualification/alpha14-note.md': 'unrelated base advancement\n',
        ARTIFACT: artifact(2)}, 'test: advance base after frozen approvals')
    root = Path(tempfile.mkdtemp(prefix='alpha14-proof-'))
    results = []
    new_prs = set()
    package = json.loads(Path('/opt/image-upgrades/package/plugin.json').read_text())
    assert package['version'] == '0.0.1-alpha.14'
    reader, action = secrets.token_hex(32), secrets.token_hex(32)
    timestamp = lambda x: x.isoformat().replace('+00:00', 'Z')
    repo = {'platform': 'github', 'name': REPOSITORY, 'baseBranch': branch, 'commit': approved_base}

    def snapshot(name):
        now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0)
        file = f'k8s/alpha14-{name}.yaml'
        key = encode(['docker.io/library/nginx', '']).decode()
        dependency_id = identity('github', REPOSITORY, branch, 'kubernetes', file, key, 'docker.io/library/nginx')
        candidate = {'id': identity(dependency_id, '1.26.3', NEW_DIGEST, 'minor'), 'newValue': '1.26.3',
            'newDigest': NEW_DIGEST, 'updateType': 'minor', 'policy': 'unknown', 'workflow': 'unknown'}
        dependency = {'id': dependency_id, 'manager': 'kubernetes', 'packageFile': file, 'dependencyKey': key,
            'packageName': 'docker.io/library/nginx', 'sourceOccurrences': 1, 'currentValue': '1.25.3',
            'currentDigest': OLD_DIGEST, 'assessment': 'updates_found', 'candidates': [candidate], 'reasons': []}
        data = {'schemaVersion': 1, 'runId': str(uuid.uuid4()), 'startedAt': timestamp(now - dt.timedelta(minutes=1)),
            'completedAt': timestamp(now), 'producer': {'renovateVersion': '44.197.0',
            'renovateImage': 'ghcr.io/renovatebot/renovate@sha256:d3aa7ee1726eb31bf0b8b006f07bcde28747a820e450cf7f4010c45913f5257f',
            'adapterVersion': '0.0.3', 'policyHash': identity('alpha14-isolated-base-recovery')}, 'repository': repo,
            'coverage': {'scope': 'Controlled candidate recovery qualification', 'status': 'complete',
                'sourceRecords': 1, 'extracted': 1, 'assessed': 1, 'excluded': 0, 'unknown': 0, 'reasons': []},
            'dependencies': [dependency]}
        raw = encode(data)
        selection = {'dependencyId': dependency_id, 'candidateId': candidate['id'], 'snapshotDigest': identity(raw.decode())}
        return raw, selection, dependency, candidate, now

    for name in ['queued', 'blocked', 'changed']:
        case = root / name
        case.mkdir(mode=0o700)
        def private(name, content):
            p = case / name
            p.write_bytes(content if isinstance(content, bytes) else content.encode())
            p.chmod(0o600)
            return str(p)
        raw, selection, dependency, candidate, now = snapshot(name)
        data_dir = case / 'requests'
        if name == 'blocked':
            data_dir.mkdir(mode=0o700)
            request_id = identity(repo, dependency['id'], candidate['id'])
            d = dict(dependency)
            receipt = {'id': request_id, 'snapshotDigest': selection['snapshotDigest'], 'repository': repo,
                'dependency': d, 'candidate': candidate, 'branch': 'balemoh-renovate/' + request_id,
                'state': 'blocked', 'reason': 'base_changed', 'createdAt': timestamp(now),
                'updatedAt': timestamp(now), 'trackingError': False}
            (data_dir / (request_id + '.json')).write_bytes(encode(receipt))
            (data_dir / (request_id + '.json')).chmod(0o600)
        if name == 'changed':
            base = commit(base, {dependency['packageFile']: manifest(name) + '# unapproved source edit\n'},
                'test: change selected source before recovery')
        env = {'PATH': os.environ['PATH'], 'HOME': str(case), 'TMPDIR': '/tmp',
            'UPGRADES_REPOSITORY': REPOSITORY, 'UPGRADES_BASE_BRANCH': branch,
            'UPGRADES_CONTROLLER_DATA_DIR': str(data_dir), 'UPGRADES_CONTROLLER_WORK_DIR': str(case / 'work'),
            'UPGRADES_CONTROLLER_SNAPSHOT_FILE': private('snapshot.json', raw),
            'UPGRADES_CONTROLLER_ADDRESS': '127.0.0.1:18443',
            'UPGRADES_CONTROLLER_READ_BEARER_FILE': private('reader', reader),
            'UPGRADES_CONTROLLER_ACTION_BEARER_FILE': private('action', action),
            'UPGRADES_CONTROLLER_GITHUB_READ_TOKEN_FILE': private('github-read', token),
            'UPGRADES_CONTROLLER_GITHUB_WRITE_TOKEN_FILE': private('github-write', token),
            'UPGRADES_CONTROLLER_PR_AUTHOR': 'github-actions[bot]', 'UPGRADES_CONTROLLER_WRITER_ENABLED': 'true',
            'UPGRADES_CONTROLLER_GENERATED_FINGERPRINT_FILE': ARTIFACT,
            'UPGRADES_CONTROLLER_TLS_CERT': str(case / 'cert.pem'), 'UPGRADES_CONTROLLER_TLS_KEY': str(case / 'key.pem')}
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
            '-keyout', str(case / 'key.pem'), '-out', str(case / 'cert.pem'), '-subj', '/CN=127.0.0.1',
            '-addext', 'subjectAltName=IP:127.0.0.1'], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        (case / 'key.pem').chmod(0o600)
        context = ssl.create_default_context(cafile=str(case / 'cert.pem'))
        client = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect(), urllib.request.HTTPSHandler(context=context))
        def call(path='/v1/requests', body=None):
            req = urllib.request.Request('https://127.0.0.1:18443' + path,
                data=None if body is None else encode(body), headers={'Authorization': 'Bearer ' + (reader if body is None else action),
                'Content-Type': 'application/json'})
            with client.open(req, timeout=15) as response:
                raw = response.read()
                return json.loads(raw) if raw else None
        stream = open(private('controller.log', b''), 'ab')
        process = None
        def start():
            nonlocal process
            process = subprocess.Popen([BINARY], env=env, stdout=stream, stderr=stream)
            until = time.monotonic() + 30
            while time.monotonic() < until:
                assert process.poll() is None, 'controller startup failed'
                try:
                    call('/healthz')
                    return
                except (OSError, ValueError):
                    time.sleep(.2)
            raise TimeoutError('controller startup deadline')
        def stop():
            if process is not None and process.poll() is None:
                process.send_signal(signal.SIGTERM)
                assert process.wait(timeout=20) == 0, 'controller shutdown failed'
        def wait_for(predicate, seconds=900):
            until = time.monotonic() + seconds
            while time.monotonic() < until:
                assert process.poll() is None, 'controller exited'
                rows = call()['requests']
                if rows:
                    r = rows[0]
                    Path('/out/latest-' + name + '.json').write_bytes(encode(r))
                    if predicate(r):
                        return r
                    assert r['state'] not in ['needs_attention', 'blocked'] or name == 'changed' or r.get('reason') in ['base_changed', 'rebase_pending'], 'request rejected: ' + r.get('reason', '')
                time.sleep(1)
            raise TimeoutError('request observation deadline')
        def exact_diff(r, expected_base, generation):
            pr = api('/pulls/' + str(r['pr']['number']))
            head = pr['head']['sha']
            assert pr['state'] == 'open' and pr['merged_at'] is None and pr['auto_merge'] is None
            assert pr['base']['ref'] == branch and pr['base']['sha'] == expected_base
            changes = api('/pulls/' + str(pr['number']) + '/files')
            assert {f['filename'] for f in changes} == {dependency['packageFile'], ARTIFACT}, 'unexpected PR files'
            def content(path, at):
                return base64.b64decode(api('/contents/' + path + '?ref=' + at)['content'])
            assert content(dependency['packageFile'], head) == manifest(name).replace(OLD, NEW).encode(), 'approved target changed'
            generated = content(ARTIFACT, head)
            assert re.sub(rb'- Source digest: `[a-f0-9]{64}`', b'- Source digest: `' + b'0' * 64 + b'`', generated) == artifact(generation).encode(), 'inventory body changed'
            checkout = case / ('verify-' + head)
            git_env = {'PATH': os.environ['PATH'], 'HOME': str(case), 'GIT_CONFIG_NOSYSTEM': '1', 'GIT_CONFIG_GLOBAL': '/dev/null',
                'GIT_CONFIG_COUNT': '1', 'GIT_CONFIG_KEY_0': 'http.https://github.com/.extraHeader',
                'GIT_CONFIG_VALUE_0': 'Authorization: Basic ' + base64.b64encode(('x-access-token:' + token).encode()).decode()}
            subprocess.run(['git', '-c', 'core.hooksPath=/dev/null', 'clone', '--depth=1', '--branch', r['branch'],
                'https://github.com/' + REPOSITORY + '.git', str(checkout)], check=True, env=git_env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            subprocess.run([BINARY, 'refresh-fingerprint', ARTIFACT], cwd=checkout, check=True, env=git_env)
            assert (checkout / ARTIFACT).read_bytes() == generated, 'fingerprint does not match actual tree'
            api('/statuses/' + head, {'state': 'success', 'context': 'fixture/alpha14-exact-diff',
                'description': 'Exact approved image and regenerated fingerprint verified', 'target_url': os.environ['RUN_URL']})
            return pr
        try:
            start()
            first = call(body=selection)
            assert call(body=selection)['id'] == first['id'], 'duplicate request changed receipt'
            if name == 'changed':
                rejected = wait_for(lambda r: r['state'] == 'blocked')
                assert rejected['reason'] == 'base_changed' and not rejected.get('pr')
                assert len(inventory()) == len(previous) + 2, 'negative request created a PR'
                results.append({'case': name, 'state': rejected['state'], 'reason': rejected['reason'], 'newPRs': 0})
                continue
            opened = wait_for(lambda r: r['state'] == 'open' and not r['trackingError'])
            assert opened['id'] == first['id'] and opened['repository'] == repo and opened['candidate']['id'] == candidate['id']
            new_prs.add(opened['pr']['number'])
            current_generation = 2 if name == 'queued' else 3
            pr = exact_diff(opened, base, current_generation)
            passing = wait_for(lambda r: r['pr']['headSha'] == pr['head']['sha'] and r['pr']['checks'] == 'passing', 90)
            before_head = passing['pr']['headSha']
            before_request = passing['id']
            before_pr = passing['pr']['number']
            current_generation += 1
            base = commit(base, {'qualification/alpha14-note.md': 'base advancement for ' + name + '\n',
                ARTIFACT: artifact(current_generation)}, 'test: advance inventory during open ' + name + ' request')
            pending = wait_for(lambda r: r.get('reason') == 'rebase_pending' and bool(r.get('rebaseHead')), 180)
            assert pending['pr']['checks'] == 'unknown' and pending['rebaseHead'] != before_head
            until = time.monotonic() + 20
            while ref(pending['branch']) != pending['rebaseHead']:
                assert time.monotonic() < until, 'pending push was not published'
                time.sleep(.2)
            archive = str(case / 'pending.tar')
            ca = private('ca-base64', base64.b64encode(ssl.PEM_cert_to_DER_cert((case / 'cert.pem').read_text())).decode())
            exported = json.loads(subprocess.check_output([BACKUP, 'export', '--repository', REPOSITORY,
                '--url', 'https://127.0.0.1:18443/v1/requests', '--token-file', str(case / 'reader'), '--ca-file', ca, '--archive', archive]))
            stop()
            env['UPGRADES_CONTROLLER_WRITER_ENABLED'] = 'false'
            restored_dir = case / 'restored'
            restored_dir.mkdir(mode=0o700)
            restored = json.loads(subprocess.check_output([BACKUP, 'restore', '--repository', REPOSITORY,
                '--archive', archive, '--destination', str(restored_dir)]))
            assert exported['records'] == restored['records'] == 1
            loaded = json.loads((restored_dir / 'state' / (pending['id'] + '.json')).read_bytes())
            assert loaded['rebaseHead'] == pending['rebaseHead'] and loaded['candidate'] == pending['candidate']
            env['UPGRADES_CONTROLLER_DATA_DIR'] = str(restored_dir / 'state')
            start()
            recovered = wait_for(lambda r: r['state'] == 'open' and not r.get('rebaseHead') and not r['trackingError'], 90)
            assert recovered['id'] == before_request and recovered['pr']['number'] == before_pr
            assert recovered['pr']['headSha'] == pending['rebaseHead']
            pr = exact_diff(recovered, base, current_generation)
            assert api('/git/commits/' + pr['head']['sha'])['parents'][0]['sha'] == base
            final = wait_for(lambda r: r['pr']['headSha'] == pr['head']['sha'] and r['pr']['checks'] == 'passing', 90)
            time.sleep(32)
            assert ref(final['branch']) == pending['rebaseHead'], 'read-only restart rewrote branch'
            try:
                call(body=selection)
                raise AssertionError('disabled writer accepted submission')
            except urllib.error.HTTPError as error:
                assert error.code == 403, 'unexpected disabled writer response'
            results.append({'case': name, 'request': final, 'oldHead': before_head, 'newHead': pending['rebaseHead'],
                'executionBase': pr['base']['sha'], 'approvedBase': approved_base,
                'rebasePreservedApprovedUpgrade': True, 'exactDiffVerified': True, 'fingerprintRecomputed': True,
                'pendingBackupRestore': True, 'readOnlyRestartNoReplay': True, 'checks': 'passing',
                'merged': False, 'automerge': False})
            print(json.dumps({'case': name, 'qualification': 'passed', 'pullRequest': pr['html_url']}), flush=True)
        finally:
            stop()
            stream.close()
    after = inventory()
    assert set(after) - set(previous) == new_prs and len(new_prs) == 2
    assert all(after[k] == v for k, v in previous.items()), 'previous PR changed'
    assert ref('main') == main, 'fixture main changed during proof'
    branches = {r['name']: r['commit']['sha'] for r in api('/branches?per_page=100')}
    assert all(branches[k] == v for k, v in old_branches.items() if k != branch), 'previous branch changed'
    receipt = {'qualification': 'passed', 'version': package['version'], 'sourceCommit': source,
        'artifactKind': 'candidate built from PR commit; not a published OCI artifact',
        'repository': REPOSITORY, 'baseBranch': branch, 'approvedBase': approved_base, 'finalBase': base,
        'fixtureMainUnchanged': True, 'previousPRsUnchanged': True, 'previousBranchesUnchanged': True,
        'credential': 'ephemeral repository-scoped GitHub Actions token; no production credential',
        'ciScope': 'harness verifies real GitHub diff and posts exact-head commit status; automatic pull_request CI is not claimed',
        'runtimeSHA256': {p: hashlib.sha256(Path(p).read_bytes()).hexdigest() for p in [BINARY, BACKUP]},
        'cases': results, 'newPRs': sorted(new_prs), 'merged': False}
    assert len(results) == 3, 'incomplete qualification'
    Path('/out/qualification.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps({'qualification': 'passed', 'newPRs': sorted(new_prs), 'merged': False}), flush=True)


if __name__ == '__main__':
    try:
        run()
    except Exception as error:
        reason = str(error) if isinstance(error, (AssertionError, TimeoutError)) else type(error).__name__
        Path('/out/failure.json').write_text(json.dumps({'qualification': 'failed', 'reason': reason}) + '\n')
        raise SystemExit('Candidate qualification failed; see controlled receipt')
