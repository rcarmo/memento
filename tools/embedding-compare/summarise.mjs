// Host-only analysis of a completed run. Raw result files are disposable.
// Usage: bun tools/embedding-compare/summarise.mjs <run-directory>
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
const root = process.argv[2];
if (!root) throw new Error('run directory required');
const read = async name => JSON.parse(await readFile(join(root, `${name}.json`), 'utf8'));
const median = xs => { const s = [...xs].sort((a,b)=>a-b); return s[Math.floor(s.length/2)]; };
const perf = [];
for (const cores of [1,4]) {
  for (const engine of ['gte','gemma']) {
    const runs = await Promise.all(['a','b'].map(order => read(`${engine}-${cores}-${order}`)));
    perf.push({engine,cores,source:runs[0].Revision,toolchain:runs[0].Toolchain,allowedCPUs:runs[0].AllowedCPUs,
      loadMS:runs.map(r=>r.LoadMS),firstEmbedMS:runs.map(r=>r.FirstEmbedMS),
      peakMiB:Math.max(...runs.flatMap(r=>r.Workloads.map(w=>w.Memory.PeakRSSBytes/2**20))),
      afterGCHeapMiB:runs.map(r=>r.AfterGCMemory.HeapBytes/2**20),
      workloads:runs[0].Workloads.map((w,i)=>{
        const ws=runs.map(r=>r.Workloads[i]); const samples=ws.flatMap(w=>w.Warm.SamplesMS).sort((a,b)=>a-b);
        const count=samples.length;
        return {name:w.Name,tokens:w.Tokens,samples:count,medianMS:median(samples),
          p95MS:samples[Math.ceil(.95*count)-1],minMS:samples[0],maxMS:samples.at(-1),
          runMedians:ws.map(w=>w.Warm.MedianMS),
          meanCPUMS:ws.reduce((a,w)=>a+(w.Warm.CPUUserMSPerOp+w.Warm.CPUSystemMSPerOp)*w.Warm.SamplesMS.length,0)/count,
          allocBytes:w.Warm.AllocBytesPerOp,allocObjects:w.Warm.AllocsPerOp,
          rssMiB:ws.map(w=>w.Memory.RSSBytes/2**20)};
      })});
  }
}
const gte=await read('gte-quality'),gemma=await read('gemma-quality');
if (gte.CorpusSHA256!==gemma.CorpusSHA256) throw new Error('corpus provenance mismatch');
const summaries=[];
for(const r of [gte,gemma]) for(const score of r.Retrieval){
  const languages={};
  for(const q of score.Ranks){const l=languages[q.Language]??={queries:0,hits1:0,hits3:0,mrr:0};l.queries++;l.hits1+=+(q.Rank===1);l.hits3+=+(q.Rank<=3);l.mrr+=1/q.Rank;}
  for(const l of Object.values(languages))l.mrr/=l.queries;
  summaries.push({engine:r.Engine,dimensions:score.Dimensions,recall1:score.Recall1,recall3:score.Recall3,recall10:score.Recall10,mrr:score.MRR,ndcg10:score.NDCG10,languages,
    failures:score.Ranks.filter(q=>q.Rank!==1).map(q=>({query:q.Query,category:q.Category,relevant:q.Relevant,rank:q.Rank,top:q.Top.slice(0,3)}))});
}
const base=gte.Retrieval[0]; const comparisons=[];
for(const s of gemma.Retrieval){
  const byID=new Map(s.Ranks.map(q=>[q.Query,q]));const topics=new Map();let gemmaOnly=0,gteOnly=0,both=0,neither=0;
  for(const q of base.Ranks){const other=byID.get(q.Query);if(!other)throw new Error('missing paired query');const a=+(q.Rank===1),b=+(other.Rank===1);if(a&&b)both++;else if(a)gteOnly++;else if(b)gemmaOnly++;else neither++;
    if(!topics.has(q.Category))topics.set(q.Category,[]);topics.get(q.Category).push(b-a);}
  const groups=[...topics.values()];let seed=20261113;const rand=()=>{seed=(Math.imul(seed,1664525)+1013904223)>>>0;return seed/2**32;};
  const bootstrap=[];for(let i=0;i<10000;i++){let sum=0,n=0;for(let j=0;j<groups.length;j++){const g=groups[Math.floor(rand()*groups.length)];sum+=g.reduce((a,b)=>a+b,0);n+=g.length;}bootstrap.push(sum/n);}bootstrap.sort((a,b)=>a-b);
  comparisons.push({gemmaDimensions:s.Dimensions,gemmaOnly,gteOnly,both,neither,recall1Difference:s.Recall1-base.Recall1,topicClusterPercentile95:[bootstrap[249],bootstrap[9749]],topicCount:groups.length});
}
console.log(JSON.stringify({corpusSHA256:gte.CorpusSHA256,description:gte.Description,performance:perf,
  retrievalCost:[gte,gemma].map(r=>({engine:r.Engine,cores:r.GOMAXPROCS,texts:r.EmbeddedTexts,wallMS:r.RetrievalEmbedMS,cpuMS:r.RetrievalCPUUserMS+r.RetrievalCPUSystemMS})),
  quality:summaries,paired:comparisons},null,2));
