#!/usr/bin/env bash
# Read-only evidence collection. Never provisions, starts services or sends alerts.
set -euo pipefail
if [[ $# -lt 1 || $# -gt 2 || ( $# == 2 && "$2" != --running ) ]]; then
  echo 'usage: phase9-preflight.sh INSTALL_ROOT [--running]' >&2
  exit 3
fi
python3 - "$@" <<'PY'
import http.client,json,pathlib,platform,subprocess,sys
root=pathlib.Path(sys.argv[1]).resolve();running=len(sys.argv)==3
report={'schema_version':1,'mode':'LOCAL_RUNNING' if running else 'LOCAL_OFFLINE',
 'architecture':platform.machine(),'phase9_complete':False,'production_validated':False,
 'external_receiver_contacted':False,'checks':[]}
def add(name,state,detail):report['checks'].append({'check':name,'state':state,'detail':detail})
host=None
try:
 p=subprocess.run([sys.executable,str(root/'bin/host-check.py'),str(root)],
  stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=60,check=False)
 if len(p.stdout)>65536:raise ValueError()
 host=json.loads(p.stdout)
 if not isinstance(host,dict) or not isinstance(host.get('checks'),list):raise ValueError()
 report['host_preflight']=host
 add('host_requirements','FAIL' if p.returncode or host.get('result')=='FAIL' else 'PASS',
     'Offline requirements checked; warnings are not production approval')
except (OSError,ValueError,subprocess.TimeoutExpired):
 add('host_requirements','FAIL','Installed host-check unavailable or invalid')
try:report['pid1']=pathlib.Path('/proc/1/comm').read_text().strip()[:64]
except OSError:report['pid1']='unknown'
cfg=None
if host and any(c.get('check')=='configuration' and c.get('state')=='PASS' for c in host['checks']):
 try:
  path=root/'etc/fleetwatch/follow.json';raw=path.read_bytes()
  if len(raw)>16384:raise ValueError()
  cfg=json.loads(raw);state=pathlib.Path(cfg['state'])
  if not state.is_absolute():state=path.parent/state
  report['configured_retention']={k:cfg[k] for k in ['max_retained_ledgers','max_retained_payload_bytes']}
  report['state_physical_bytes']=state.stat().st_size if state.is_file() else None
 except (OSError,ValueError,KeyError,TypeError):add('configuration_summary','FAIL','Could not collect bounded config/state metadata')
if running:
 try:
  if cfg is None:raise ValueError()
  host,port=cfg['listen'].rsplit(':',1)
  if host!='127.0.0.1' or not port.isdecimal() or not 1<=int(port)<=65535 or str(int(port))!=port:raise ValueError()
  observations={}
  for route in ['readyz','status']:
   connection=http.client.HTTPConnection('127.0.0.1',int(port),timeout=5)
   try:
    connection.request('GET','/'+route);response=connection.getresponse();body=response.read(65537)
    if len(body)>65536:raise ValueError()
    status=json.loads(body)
    if not isinstance(status,dict):raise ValueError()
    observations[route]={'http_status':response.status,**{k:status.get(k) for k in [
     'schema_version','mode','code','ready','processed_through','retained_ledgers',
     'retained_payload_bytes','pending_findings','gap_episodes','last_successful_cycle_unix','final_verdict']}}
   finally:connection.close()
  report['loopback_observations']=observations
  ready=observations['readyz']
  add('current_readiness','PASS' if ready['http_status']==200 and ready['ready'] is True and ready['final_verdict']=='UNDETERMINED' else 'FAIL',
      'One point-in-time loopback observation; not uptime, source authentication or receiver proof')
 except (OSError,ValueError,KeyError,TypeError,http.client.HTTPException):
  add('current_readiness','FAIL','Bounded loopback status unavailable or invalid')
else:add('current_readiness','UNVERIFIED','Use --running on the chosen host after starting the reviewed service')
for name in ['zero_cost_account_or_owned_host','persistent_disk_and_retention','logout_and_reboot',
 'authorized_receiver_delivery','backup_restore_and_resume','representative_load_and_capacity']:
 add(name,'UNVERIFIED','Requires target-host/operator evidence recorded in PHASE9_ACCEPTANCE.md')
report['result']='REQUIREMENTS_FAILED' if any(c['state']=='FAIL' for c in report['checks']) else 'COLLECTED_NOT_APPROVED'
print(json.dumps(report,indent=2))
raise SystemExit(1 if report['result']=='REQUIREMENTS_FAILED' else 0)
PY
