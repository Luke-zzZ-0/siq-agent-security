import subprocess,os,pathlib,hashlib,json
root=pathlib.Path('/tmp/siq-n01-builds');root.mkdir(exist_ok=True);rows=[]
for target in ['linux/arm64','linux/amd64','darwin/arm64','windows/amd64']:
 goos,arch=target.split('/');out=root/('agentshield-'+goos+'-'+arch+('.exe' if goos=='windows' else ''))
 env=os.environ.copy();env.update(GOOS=goos,GOARCH=arch,CGO_ENABLED='0')
 cmd=['go','build','-ldflags','-X main.Version=n01-validation','-o',str(out),'./cmd/agentshield']
 p=subprocess.run(cmd,env=env,capture_output=True,text=True)
 rows.append({'target':target,'command':cmd,'CGO_ENABLED':0,'exit_code':p.returncode,'sha256':hashlib.sha256(out.read_bytes()).hexdigest() if p.returncode==0 else None,'output':p.stdout+p.stderr});print(target,p.returncode,flush=True)
 if p.returncode:break
(root/'build-results.json').write_text(json.dumps(rows,indent=2)+'\n')
assert len(rows)==4 and all(x['exit_code']==0 for x in rows)
