from __future__ import annotations
import difflib,json,sys
CASES=[("",""),("a",""),("abc","abc"),("abc","acb"),("abcd","bc"),("tide","diet"),("diet","tide"),("Straße".casefold(),"STRASSE".casefold()),("界面","界线"),("a"*199+"b","a"*199+"c"),("x"*210+"a","x"*210+"b"),("abababab","babababa"),("project alpha","project alfa"),("notes","note")]
print(json.dumps([{"a":a,"b":b,"ratio":difflib.SequenceMatcher(a=a,b=b).ratio()} for a,b in CASES],ensure_ascii=True,indent=2))
