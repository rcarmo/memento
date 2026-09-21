import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
// Browser module is plain JS under the embedded asset directory, not a Node
// package. Import its exact bytes without depending on package-type defaults.
const source = await readFile(new URL('../../internal/service/graph_static/diagnostics.js', import.meta.url), 'utf8');
const { scopedDiagnostics, diagnosticTargets, relationshipSummary } = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
const a = { id: 'a', path: '/a.md' }, b = { id: 'b', path: '/b.md' };
const diag = { id: 'd', concept_ids: ['a', 'b'], rule: 'exact_duplicate', message: 'shared finding' };
test('selection and filtered view never expose other diagnostic targets', () => {
 const graph = {mode:'direct',nodes:[a,b],diagnostics:[diag,diag]};
 const got = scopedDiagnostics(graph,a,[a,b]);
 assert.equal(got.length,1);assert.deepEqual(got[0].concept_ids,['a']);assert(got[0].scope_limited);
 assert.deepEqual(scopedDiagnostics(graph,null,[b])[0].concept_ids,['b']);
 assert.deepEqual(scopedDiagnostics(graph,null,[]),[]);
 assert.deepEqual(diag.concept_ids,['a','b']);
 assert.deepEqual(diagnosticTargets(got[0],[a]),[{id:'a',label:'/a.md'}]);
});
test('filtered aggregate view maps only visible cluster membership', () => {
 const ca={id:'ca',member_count:1},cb={id:'cb',member_count:1};
 const graph={mode:'aggregated',clusters:[ca,cb],nodes:[],memberships:[['a','ca'],['b','cb']],diagnostics:[diag]};
 assert.deepEqual(scopedDiagnostics(graph,null,[ca])[0].concept_ids,['a']);
 assert.deepEqual(scopedDiagnostics(graph,cb,[cb])[0].concept_ids,['b']);
 assert.deepEqual(scopedDiagnostics({...graph,mode:'direct'},ca,[a])[0].concept_ids,['a']);
});
test('unavailable/loading data is distinct from zero and truncated lists use degree totals',()=>{
 assert.match(relationshipSummary({loading:true}),/Loading/);
 assert.match(relationshipSummary({error:'failed'}),/unavailable/);
 assert.match(relationshipSummary({node:a}),/unavailable/);
 const resolved={kind:'explicit',resolution:'resolved',source:'a',target:'b'};
 assert.match(relationshipSummary({node:{explicit_in_degree:8,explicit_out_degree:12},inbound:[],outbound:[resolved,{resolution:'asset'},{resolution:'external'},{resolution:'broken'}],truncated:true}),/^8 inbound \/ 12 outbound concept links · 1 external · 1 assets · 1 unresolved · list truncated$/);
 assert.match(relationshipSummary({inbound:[],outbound:[]}),/^0 inbound \/ 0 outbound/);
 assert.deepEqual(diagnosticTargets({concept_ids:['x','x']},[]),[{id:'x',label:'Open memory x'}]);
});
