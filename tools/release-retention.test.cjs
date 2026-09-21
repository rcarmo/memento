const {test}=require('node:test');
const assert=require('node:assert/strict');
const {retentionPlan}=require('./release-retention.cjs');
const releases=Array.from({length:9},(_,i)=>({id:i,tag_name:`v1.0.${i}`,created_at:new Date(1700000000000+i*1000).toISOString()}));
test('live and rollback releases survive count retention',()=>{
 const all=[...releases,{id:100,tag_name:'v0.5.9',created_at:'2020-01-01'}, {id:101,tag_name:'runtime-models-v1',created_at:'2020-01-01'}];
 const p=retentionPlan(all,[]);
 assert.deepEqual(p.tags,['v1.0.3','v1.0.1','v1.0.0']);
 assert(!p.releaseIDs.includes(100));assert(!p.releaseIDs.includes(101));
});
test('retained old multiarch children and arbitrary pinned digests are never pruned',()=>{
 const p=retentionPlan(releases,[]);
 assert.deepEqual(p.packageVersionIDs,[]);
});
test('failed/manual runs cannot evict retained release evidence',()=>{
 const runs=[{id:1,event:'push',status:'completed',head_branch:'v1.0.8'},
 {id:2,event:'push',status:'completed',head_branch:'v1.0.0'},
 {id:3,event:'push',status:'in_progress',head_branch:'v1.0.0'},
 ...Array.from({length:20},(_,i)=>({id:10+i,event:'workflow_dispatch',status:'completed',head_branch:'main'}))];
 assert.deepEqual(retentionPlan(releases,runs).runIDs,[2]);
});
test('explicit protected refs and small inventories',()=>{
 assert.deepEqual(retentionPlan([],[]).releaseIDs,[]);
 assert(!retentionPlan(releases,[],1,['v1.0.0']).tags.includes('v1.0.0'));
});
