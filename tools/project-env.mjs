// Bootstrap direct Node/Bun helpers through the same validated shell resolver.
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
const script = fileURLToPath(new URL('./project-env.sh', import.meta.url));
const fields = ['PROJECT_TMP_ROOT','PROJECT_ORIGINAL_TMPDIR','MEMENTO_RUN_ROOT','BUILD_ROOT','TMPDIR','TMP','TEMP','GOTMPDIR','GOCACHE','GOMODCACHE','GOTOOLCHAIN','XDG_CACHE_HOME','BUN_INSTALL_CACHE_DIR','npm_config_cache','PIP_CACHE_DIR','UV_CACHE_DIR','PYTHONPYCACHEPREFIX','PLAYWRIGHT_BROWSERS_PATH','TOOLS_DIR','PROFILE_ROOT'];
const lines = execFileSync('bash', [script, 'bash', '-c', 'for key in "$@"; do printf "%s=%s\\0" "$key" "${!key}"; done', '--', ...fields], {encoding:'utf8'}).split('\0');
for (const line of lines) {const i=line.indexOf('=');if(i>0)process.env[line.slice(0,i)]=line.slice(i+1);}
