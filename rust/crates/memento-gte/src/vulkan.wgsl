// FP32 kernels for the existing GTE1 tensor layout. No subgroups, FP16 or
// cooperative-matrix requirements: use ordinary Vulkan compute workgroups.
struct Params { m:u32, n:u32, k:u32, seq:u32, heads:u32, hidden:u32, pad0:u32, pad1:u32 }
@group(0) @binding(0) var<storage,read> a:array<f32>;
@group(0) @binding(1) var<storage,read> b:array<f32>;
@group(0) @binding(2) var<storage,read> c:array<f32>;
@group(0) @binding(3) var<storage,read_write> out:array<f32>;
@group(0) @binding(4) var<uniform> p:Params;
var<workgroup> ta:array<f32,128>;
var<workgroup> tb:array<f32,128>;

// a[M,K] * b[N,K]^T + c[N], with 8x8 output and K=16 shared tiles.
@compute @workgroup_size(8,8)
fn linear(@builtin(local_invocation_id) l:vec3<u32>, @builtin(workgroup_id) g:vec3<u32>) {
 let row=g.y*8+l.y; let col=g.x*8+l.x; var sum=0.0;
 for(var base=0u;base<p.k;base+=16u) {
  for(var j=0u;j<2u;j++) {
   let ka=l.x+j*8u; let kb=l.y+j*8u;
   ta[l.y*16u+ka]=0.0; tb[l.x*16u+kb]=0.0;
   if(row<p.m && base+ka<p.k) { ta[l.y*16u+ka]=a[row*p.k+base+ka]; }
   if(col<p.n && base+kb<p.k) { tb[l.x*16u+kb]=b[col*p.k+base+kb]; }
  }
  workgroupBarrier();
  for(var k=0u;k<16u;k++) { sum+=ta[l.y*16u+k]*tb[l.x*16u+k]; }
  workgroupBarrier();
 }
 if(row<p.m && col<p.n) { out[row*p.n+col]=sum+c[col]; }
}
@compute @workgroup_size(64)
fn norm(@builtin(global_invocation_id) id:vec3<u32>) {
 let row=id.x; if(row>=p.m) {return;} let h=p.hidden; var mean=0.0;
 for(var j=0u;j<h;j++){mean+=a[row*h+j];} mean/=f32(h);
 var variance=0.0;
 for(var j=0u;j<h;j++){let v=a[row*h+j]-mean;variance+=v*v;}
 let inv=inverseSqrt(variance/f32(h)+1e-12);
 for(var j=0u;j<h;j++){out[row*h+j]=(a[row*h+j]-mean)*inv*b[j]+c[j];}
}
@compute @workgroup_size(64)
fn residual(@builtin(global_invocation_id) id:vec3<u32>) {
 if(id.x<p.m){out[id.x]=a[id.x]+b[id.x];}
}
@compute @workgroup_size(64)
fn gelu(@builtin(global_invocation_id) id:vec3<u32>) {
 if(id.x>=p.m){return;} let x=a[id.x];
 out[id.x]=0.5*x*(1.0+tanh(0.7978846*(x+0.044715*x*x*x)));
}
// One batch/head slice of scores [seq,seq]. c is the original padding mask.
@compute @workgroup_size(8,8)
fn scores(@builtin(global_invocation_id) id:vec3<u32>) {
 let q=id.y;let key=id.x;let bh=id.z;if(q>=p.seq||key>=p.seq){return;}
 let batch=bh/p.heads;let head=bh%p.heads;let hd=p.hidden/p.heads;
 let qi=(batch*p.seq+q)*p.hidden+head*hd;let ki=(batch*p.seq+key)*p.hidden+head*hd;
 var s=-10000.0;
 if(c[batch*p.seq+key]>0.0){s=0.0;for(var d=0u;d<hd;d++){s+=a[qi+d]*b[ki+d];}s/=sqrt(f32(hd));}
 out[(bh*p.seq+q)*p.seq+key]=s;
}
@compute @workgroup_size(64)
fn softmax(@builtin(global_invocation_id) id:vec3<u32>) {
 let row=id.x;if(row>=p.m){return;}var largest=-3.402823e38;
 for(var j=0u;j<p.seq;j++){largest=max(largest,a[row*p.seq+j]);}
 var sum=0.0;for(var j=0u;j<p.seq;j++){sum+=exp(a[row*p.seq+j]-largest);}
 for(var j=0u;j<p.seq;j++){out[row*p.seq+j]=exp(a[row*p.seq+j]-largest)/sum;}
}
@compute @workgroup_size(8,8)
fn values(@builtin(global_invocation_id) id:vec3<u32>) {
 let d=id.x;let q=id.y;let batch=id.z;if(d>=p.hidden||q>=p.seq){return;}
 let head=d/(p.hidden/p.heads);let row=(batch*p.heads+head)*p.seq+q;var s=0.0;
 if(c[batch*p.seq+q]>0.0){for(var key=0u;key<p.seq;key++){s+=a[row*p.seq+key]*b[(batch*p.seq+key)*p.hidden+d];}}
 out[(batch*p.seq+q)*p.hidden+d]=s;
}
