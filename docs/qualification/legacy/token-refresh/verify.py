"""Exercise and recreate only the disposable, emptyDir-backed diagnostic Pod."""
import json,subprocess,time,hashlib
from pathlib import Path
CTX=['kubectl','--context','admin@talos-k8s-homelab','-n','balemoh']
NAME='renovate-tracking-refresh-proof-20261101'
BASE=Path('.local/renovate-request-readiness')

def cmd(args, *, check=True):
    return subprocess.run(CTX+args,text=True,capture_output=True,check=check).stdout

def pod():return json.loads(cmd(['get','pod',NAME,'-o','json']))
def consumer():return json.loads(cmd(['exec',NAME,'-c','server','--','cat','/work/consumer.json']))
def wait(predicate,timeout=100):
    stop=time.monotonic()+timeout
    while time.monotonic()<stop:
        try:
            result=predicate()
            if result:return result
        except (subprocess.CalledProcessError,json.JSONDecodeError,KeyError):pass
        time.sleep(2)
    raise RuntimeError('proof condition timed out')
def status(obj,sidecar=False):
    return obj['status']['initContainerStatuses' if sidecar else 'containerStatuses'][0]
def ready(obj):return any(c['type']=='Ready' and c['status']=='True' for c in obj['status']['conditions'])

def capture(stage):
    value=consumer();obj=pod()
    assert value['writerEnabled'] is False and value['actionStatus']==403
    result={'stage':stage,'podUID':obj['metadata']['uid'],'mainContainerID':status(obj)['containerID'],'mainRestarts':status(obj)['restartCount'],'refresherContainerID':status(obj,True)['containerID'],'refresherRestarts':status(obj,True)['restartCount'],'consumer':value}
    print(json.dumps(result),flush=True)
    return result

assert pod()['metadata']['labels']['app.kubernetes.io/name']=='renovate-tracking-refresh-proof'
assert not pod()['metadata'].get('ownerReferences')
assert 'argocd.argoproj.io/tracking-id' not in pod()['metadata'].get('annotations', {})
assert not any('persistentVolumeClaim' in v for v in pod()['spec']['volumes'])
wait(lambda:consumer()['observedTokens']>=2)
initial=capture('scheduled_refresh')
# Restart only our diagnostic sidecar. The controller container remains running.
cmd(['exec',NAME,'-c','github-read-token','--','python3','-c','import os,signal;os.kill(1,signal.SIGTERM)'],check=False)
wait(lambda:status(pod(),True)['restartCount']>initial['refresherRestarts'] and ready(pod()) and consumer()['observedTokens']>initial['consumer']['observedTokens'])
sidecar=capture('refresher_restart')
assert sidecar['mainContainerID']==initial['mainContainerID']
# Invalid injected token is synthetic, not an expired real token.
before=consumer()
cmd(['exec',NAME,'-c','github-read-token','--','python3','-c',"from pathlib import Path;Path('/github-read/token').write_text('invalid-fixture-token')"])
wait(lambda:consumer()['unavailableReads']>before['unavailableReads'])
failure=capture('invalid_token_unknown')
assert failure['consumer']['checks']=='unknown'
wait(lambda:consumer()['checks']=='passing' and consumer()['observedTokens']>before['observedTokens'])
recovered=capture('renewal_recovers_tracking')
cmd(['exec',NAME,'-c','server','--','sh','-c','kill -TERM 1'],check=False)
wait(lambda:status(pod())['restartCount']>recovered['mainRestarts'] and status(pod())['containerID']!=recovered['mainContainerID'] and ready(pod()) and consumer()['checks']=='passing')
main=capture('controller_restart')
assert main['refresherContainerID']==recovered['refresherContainerID']
for role in ['server','github-read-token']:
    (BASE/('refresh-'+role+'.log')).write_text(cmd(['logs',NAME,'-c',role]))
oldUID=pod()['metadata']['uid']
cmd(['delete','pod',NAME,'--wait=true','--timeout=45s'])
cmd(['create','-f','docs/specs/spec-001-balemoh-image-upgrades/poc/token-refresh/proof.json'])
cmd(['wait','--for=condition=Initialized','pod/'+NAME,'--timeout=60s'])
wait(lambda: 'running' in status(pod()).get('state', {}))
cmd(['cp',str(BASE/'refresh-head'),NAME+':/work/head','-c','server'])
cmd(['cp',str(BASE/'controller-refresh-proof'),NAME+':/work/controller-probe','-c','server'])
wait(lambda:ready(pod()) and consumer()['checks']=='passing')
replacement=capture('pod_recreation')
assert replacement['podUID']!=oldUID
assert replacement['mainRestarts']==0 and replacement['refresherRestarts']==0
receipt={'pluginSourceCommit':'e13e71d62ebdd0c4173d4e4ed1b8479262fa0c75','binarySHA256':hashlib.sha256((BASE/'controller-refresh-proof').read_bytes()).hexdigest(),'renewalSecondsInProof':30,'renewalSecondsInProduction':2400,'stages':[initial,sidecar,failure,recovered,main,replacement],'scope':'Pinned controller code in isolated diagnostic Pod; real read-only GitHub calls; invalid token fault is synthetic','repositoryWrites':0,'writerEnabled':False}
Path('docs/specs/spec-001-balemoh-image-upgrades/token-refresh-receipt.json').write_text(json.dumps(receipt,indent=2)+'\n')
print('All refresh and restart phases passed; receipt saved.',flush=True)
