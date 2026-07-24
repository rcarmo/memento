import { serve } from "bun";
import { extname, join, normalize } from "node:path";

const root = join(import.meta.dir, "..", "src", "memento", "graph_debug", "static");
const port = Number(process.env.GRAPH_FIXTURE_PORT || 18765);
const count = Number(process.env.GRAPH_FIXTURE_NODES || 120);
const mode = process.env.GRAPH_FIXTURE_MODE || (count > 2000 ? "aggregated" : "direct");
const types = ["project", "instance", "person", "service", "system", "skill"];
const namespaceFor=(type:string)=>type==="person"?"/people/":type==="skill"?"/skills/":`/${type}s/`;
const nodes = Array.from({length: Math.min(count, 2000)}, (_, i) => {const type=types[i%types.length],namespace=namespaceFor(type);return{
  id:`node-${i}`, path:`${namespace}node-${i}.md`, title:`Memory ${i}`, type, status:"active", tags:["fixture",`group-${i%12}`], namespace, updated_at:"2026-07-20T00:00:00Z", updated_by:i%3?"piclaw":"curator", markdown_bytes:200+i*3, asset_bytes:i%8===0?5000:0, combined_bytes:200+i*3+(i%8===0?5000:0), explicit_in_degree:2, explicit_out_degree:2, broken_link_count:i%97===0?1:0, orphan:i%113===0, proposal_count:i%17===0?1:0, pending_proposal_count:i%53===0?1:0, embedding:{status:i%19===0?"missing":"ready",model_id:"gte-small",dimensions:384,embedding_revision:"fixture-rev"}, coarse_position:{x:Math.cos(i*.41)*(2+Math.sqrt(i)*.18),y:Math.sin(i*.41)*(2+Math.sqrt(i)*.18),z:((i%17)-8)*.12}, anomaly_ids:i%97===0?[`diagnostic:broken:${i}`]:[]
};});
const edges = Array.from({length: Math.max(0,nodes.length-1)},(_,i)=>({id:`edge-${i}`,source:`node-${i}`,target:`node-${i+1}`,raw_target:`/projects/node-${i+1}.md`,kind:"explicit",canonical:true,resolution:"resolved",first_seen_revision:"fixture-rev",last_checked_revision:"fixture-rev"}));
const diagnostics = nodes.filter(n=>n.broken_link_count).map(n=>({id:n.anomaly_ids[0],rule:"broken_links",severity:"error",concept_ids:[n.id],message:"One explicit link target does not resolve.",measured:{broken_link_count:1},threshold:{maximum:0},derived:false}));
const clusters = Array.from({length:12},(_,i)=>({id:`cluster-${i}`,label:`Namespace ${i}`,namespace:`/namespace-${i}/`,member_count:Math.ceil(count/12),markdown_bytes:count*250,asset_bytes:count*50,combined_bytes:count*300,explicit_in_degree:20,explicit_out_degree:20,broken_link_count:i===0?2:0,orphan_count:0,type_counts:[["project",10]],status_counts:[["active",10]],coarse_position:{x:Math.cos(i/12*Math.PI*2)*6,y:Math.sin(i/12*Math.PI*2)*6,z:(i%3)-1}}));
const clusterEdges=clusters.map((c,i)=>({id:`ce-${i}`,source:c.id,target:clusters[(i+1)%clusters.length].id,explicit_edge_count:5,canonical:true}));
const overview={schema_version:1,mode,revisions:{repository:"fixture-rev",index:"fixture-rev",embedding:"fixture-rev",stale:false},metrics:{memory_count:count,markdown_bytes:count*250,asset_bytes:count*50,explicit_edges:edges.length,broken_edges:diagnostics.length,orphan_count:1},nodes:mode==="direct"?nodes:[],edges:mode==="direct"?edges:[],clusters:mode==="aggregated"?clusters:[],cluster_edges:mode==="aggregated"?clusterEdges:[],memberships:[],layout_seed:"fixture-rev",layout_version:"v1",diagnostics,truncated:mode==="aggregated"};
const detail=(id:string)=>{const n=nodes.find(n=>n.id===id)||nodes[0];return{schema_version:1,revisions:overview.revisions,node:n,preview:"This is a bounded Markdown preview for visual regression and inspector interaction.",preview_truncated:false,outbound:edges.filter(e=>e.source===n.id),inbound:edges.filter(e=>e.target===n.id),assets:[{asset_kind:"diagram",version:"1.0.0",metadata_bytes:320,payload_bytes:4096,source_proposal_id:"proposal-1"}],proposals:[{proposal_id:"proposal-1",author:"piclaw",status:"submitted",intent:"Update the memory",base_revision:"fixture-rev",created_at:"2026-07-20T00:00:00Z",updated_at:"2026-07-20T00:00:00Z"}]};};
const typesByExt:any={".html":"text/html",".css":"text/css",".js":"text/javascript",".json":"application/json"};
const principals=[
 {name:"projects-reader",roles:["reader"],read_prefixes:["/projects/"],write_prefixes:[]},
 {name:"shared-reader",roles:["reader"],read_prefixes:["/projects/","/skills/"],write_prefixes:[]},
];
const scopedNodes=(req:Request)=>{const name=req.headers.get("X-Memento-Simulated-Principal")||"";const principal=principals.find(item=>item.name===name);if(!name)return nodes;if(!principal)return null;return nodes.filter(node=>principal.read_prefixes.some(prefix=>node.path.startsWith(prefix)));};
const scopedOverview=(req:Request)=>{const visible=scopedNodes(req);if(visible===null)return null;if(visible===nodes)return overview;const ids=new Set(visible.map(node=>node.id));const visibleEdges=edges.filter(edge=>ids.has(edge.source)&&ids.has(edge.target));return{...overview,mode:"direct",nodes:visible,edges:visibleEdges,clusters:[],cluster_edges:[],metrics:{...overview.metrics,memory_count:visible.length,explicit_edges:visibleEdges.length,broken_edges:0,orphan_count:0},truncated:false};};
serve({
 port,
 async fetch(req){
  const url=new URL(req.url);let p=url.pathname;
  if(p==="/graph")p="/index.html";
  if(p==="/graph/api/v1/principals")return Response.json({schema_version:1,principals});
  const visible=scopedNodes(req);if(visible===null)return Response.json({error:"unknown simulated principal"},{status:400});
  if(p==="/graph/api/v1/overview")return Response.json(scopedOverview(req));
  if(p==="/graph/api/v1/search"&&req.method==="POST"){
   const body=await req.json().catch(()=>({})) as {query?:string};const q=(body.query||"").toLowerCase();
   const results=visible.filter(n=>`${n.title} ${n.path} ${n.tags.join(" ")} ${detail(n.id).preview}`.toLowerCase().includes(q)).slice(0,20).map(n=>({id:n.id,path:n.path,title:n.title,type:n.type,tags:n.tags,snippet:detail(n.id).preview}));
   return Response.json({schema_version:1,results});
  }
  if(p.startsWith("/graph/api/v1/memories/")){const id=decodeURIComponent(p.split("/").pop()!);if(!visible.some(n=>n.id===id))return new Response("not found",{status:404});return Response.json(detail(id));}
  if(p.startsWith("/graph/api/v1/neighbourhood/")){const id=decodeURIComponent(p.split("/").pop()!);const center=visible.findIndex(n=>n.id===id);if(center<0)return new Response("not found",{status:404});const slice=visible.slice(Math.max(0,center-1),Math.min(visible.length,center+2));const ids=new Set(slice.map(n=>n.id));return Response.json({schema_version:1,revisions:overview.revisions,center_id:id,nodes:slice,edges:edges.filter(e=>ids.has(e.source)&&ids.has(e.target)),depth:1});}
  if(p.startsWith("/graph/api/v1/clusters/")){const ids=new Set(visible.map(n=>n.id));return Response.json({schema_version:1,revisions:overview.revisions,cluster_id:p.split("/").pop(),parent_position:{x:0,y:0,z:0},nodes:visible,edges:edges.filter(e=>ids.has(e.source)&&ids.has(e.target)),next_cursor:null});}
  if(p.startsWith("/graph/api/v1/embeddings/"))return Response.json({available:true,running:false,pending:false,last_error:null,last_scope:"visible",queued_paths:visible.length,repository_revision:"fixture-rev"},{status:req.method==="POST"?202:200});
  if(p.startsWith("/graph/api/v1/export/"))return Response.json({ok:true});
  if(!p.startsWith("/graph/assets/")&&p!=="/index.html")return new Response("not found",{status:404});
  const rel=p==="/index.html"?"index.html":p.replace("/graph/assets/","");if(rel.includes(".."))return new Response("not found",{status:404});const file=Bun.file(join(root,rel));if(!(await file.exists()))return new Response("not found",{status:404});let content=await file.text();if(rel==="index.html")content=content.replaceAll("__GRAPH_PREFIX__","/graph");return new Response(content,{headers:{"Content-Type":typesByExt[extname(rel)]||"application/octet-stream","Cache-Control":"no-store"}});
 }
});
console.log(`graph fixture listening on http://127.0.0.1:${port}/graph (${mode}, ${count})`);
