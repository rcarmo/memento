#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/project-env.sh" || exit 1
# Build a separately profiled embedding worker for model-test only. Release
# builds never use this overlay or its environment switch.
set -euo pipefail
root=$(cd -- "$(dirname "$0")/.." && pwd)
cd "$root"
GO=${GO:-go}
mkdir -p "$BUILD_ROOT"
work=$(mktemp -d "$MEMENTO_RUN_ROOT/profile-worker-XXXXXX")
source="$root/cmd/memento-embed-go/main.go"
sed 's/func main() {/func main() { exit = profiledWorkerExit;/' "$source" > "$work/main.go.txt"
cat > "$work/hook.go.txt" <<'GO'
package main
import("fmt";"os";"path/filepath";"runtime";"runtime/pprof";"strings")
var workerProfileDir string
func init(){
 root:=os.Getenv("MEMENTO_TEST_PROFILE_DIR");if root==""{return}
 workerProfileDir=filepath.Join(root,fmt.Sprintf("worker-%d",os.Getpid()))
 if err:=os.MkdirAll(workerProfileDir,0755);err!=nil{panic(err)}
 cpu,err:=os.Create(filepath.Join(workerProfileDir,"cpu.pprof"));if err!=nil{panic(err)}
 if err=pprof.StartCPUProfile(cpu);err!=nil{panic(err)}
 exe,_:=os.Executable()
 var meta strings.Builder
 fmt.Fprintf(&meta,"pid=%d\nparent=%d\nkind=model-worker\npath=%s\ncpuprofile=%s\nmemprofile=%s\n",os.Getpid(),os.Getppid(),exe,filepath.Join(workerProfileDir,"cpu.pprof"),filepath.Join(workerProfileDir,"heap.pprof"))
 if err=os.WriteFile(filepath.Join(workerProfileDir,"metadata.txt"),[]byte(meta.String()),0644);err!=nil{panic(err)}
}
func profiledWorkerExit(code int){
 if workerProfileDir!=""{
  pprof.StopCPUProfile();runtime.GC()
  file,err:=os.Create(filepath.Join(workerProfileDir,"heap.pprof"));if err!=nil{panic(err)}
  if err=pprof.Lookup("allocs").WriteTo(file,0);err!=nil{panic(err)}
  if err=file.Close();err!=nil{panic(err)}
 }
 os.Exit(code)
}
GO
printf '{"Replace":{"%s":"%s","%s":"%s"}}\n' "$source" "$work/main.go.txt" "$root/cmd/memento-embed-go/zz_profile_worker.go" "$work/hook.go.txt" > "$work/overlay.json"
CGO_ENABLED=0 "$GO" build -overlay="$work/overlay.json" -o "$BUILD_ROOT/memento-embed-go" ./cmd/memento-embed-go
printf 'Profiled model-test worker overlay: %s\n' "$work"
