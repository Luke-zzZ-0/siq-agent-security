from pathlib import Path
import hashlib,json,os,stat,subprocess,tempfile
old=Path('/tmp/siq-ornith-fix-builds/agentshield-linux-arm64')
new=Path('/tmp/siq-n01-builds/agentshield-linux-arm64')
rows=[]
def snapshot(root):
    data={}
    for p in sorted(root.rglob('*')):
        mode=p.lstat().st_mode
        content=os.readlink(p) if stat.S_ISLNK(mode) else hashlib.sha256(p.read_bytes()).hexdigest() if stat.S_ISREG(mode) else ''
        data[str(p.relative_to(root))]=[mode,content]
    return data
def run(binary,root,args,expect=0):
    env={k:v for k,v in os.environ.items() if not k.startswith(('SIQ_AGENT_SECURITY_','AGENTSHIELD_'))}
    env['SIQ_AGENT_SECURITY_STATE_DIR']=str(root)
    p=subprocess.run([str(binary),*args],env=env,capture_output=True,text=True,timeout=30)
    if expect==0:assert p.returncode==0,(args[0],p.returncode,p.stderr)
    else:assert p.returncode!=0,(args[0],'unexpected success')
    return p
with tempfile.TemporaryDirectory(prefix='siq-n01-native-') as base:
    base=Path(base);root=base/'state'
    run(old,root,['init'])
    run(old,root,['pubkey'])
    legacy=json.loads((root/'state-format.json').read_text());assert legacy['format_version']==1
    skill=base/'fixture-skill';skill.mkdir();(skill/'SKILL.md').write_text('---\nname: n01-fixture\ndescription: Read a local fixture for compatibility testing.\n---\nA harmless fixture.\n')
    admission=json.loads(run(old,root,['admit',str(skill)]).stdout)
    grant=json.loads(run(old,root,['grant',admission['admission_id'],'--platform','hermes','--subject','n01-native-fixture']).stdout)
    revoked=json.loads(run(old,root,['grant','revoke',grant['grant']['grant_id']]).stdout)
    assert revoked['grant']['status']=='revoked'
    original=snapshot(root)
    result=json.loads(run(new,root,['state-migrate','--confirm']).stdout)
    assert result['status']=='migrated' and result['format_version']==2
    after=snapshot(root)
    for path,data in original.items():
        if path!='state-format.json':assert after[path]==data,('historical object modified',path)
    assert not (root/'logs/migration-plan.json').exists()
    plan=json.loads((root/'state-migration-v2/plan.json').read_text())
    for entry in plan['entries']:
        p=root/'state-migration-v2/backup'/entry['path']
        assert p.exists(),entry['path']
        if not entry['directory']:assert hashlib.sha256(p.read_bytes()).hexdigest()==entry['sha256']
    rows.append(dict(case='real-old-v1-to-new-v2',status='passed',historical_entries_preserved=len(original)-1,backup_entries=len(plan['entries']),revoked_grant_revision_preserved=revoked['state_revision']))
    # The old compatibility-aware executable, not a mocked version constant.
    for command in [['init'],['pubkey'],['serve'],['grant','revoke',grant['grant']['grant_id']]]:
        before=snapshot(root);p=run(old,root,command,1)
        assert 'incompatible state directory' in p.stderr and snapshot(root)==before
        rows.append(dict(case='actual-old-binary-refuses-v2',command=command[0],zero_writes=True))
    before=snapshot(root)
    status=json.loads(run(new,root,['state-status']).stdout);assert status['compatible'] is True
    repeat=json.loads(run(new,root,['state-migrate','--confirm']).stdout);assert repeat['status']=='up_to_date' and snapshot(root)==before
    # New code reads the preserved signed latest grant as revoked; no new revision.
    p=run(new,root,['grant','revoke',grant['grant']['grant_id']],1)
    assert 'revoked' in p.stderr and snapshot(root)==before
    rows.append(dict(case='new-reader-preserves-revoked-state',zero_writes=True))
    marker=json.loads((root/'state-format.json').read_text());marker['min_writer']=3;(root/'state-format.json').write_text(json.dumps(marker))
    before=snapshot(root);status=json.loads(run(new,root,['state-status']).stdout)
    assert not status['compatible'] and 'state-status' in status['recovery']
    run(new,root,['init'],1);assert snapshot(root)==before
    rows.append(dict(case='future-writer-and-recovery-guidance',zero_writes=True))
Path('/tmp/siq-n01-native.json').write_text(json.dumps({'platform':'linux/arm64','old_binary_sha256':hashlib.sha256(old.read_bytes()).hexdigest(),'old_binary_source':'reviewed v1 guard baseline built before this N01 implementation; not a published release','new_binary_sha256':hashlib.sha256(new.read_bytes()).hexdigest(),'new_binary_version':'n01-validation','results':rows},indent=2)+'\n')
print('Native two-binary migration and revoked-grant preservation passed:',len(rows))
