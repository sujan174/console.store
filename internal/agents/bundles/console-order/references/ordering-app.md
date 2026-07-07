# The ordering app — one fixed template, stitched by intent

This is the whole ordering experience as **one self-contained widget**: search →
menu (categories) → item → customize → cart → checkout, all navigated
client-side. You do NOT compose this from parts or regenerate it per turn — you
**copy the template verbatim, fill one `DATA` object, and render once.** That is
what makes it fast and identical every time.

Read `surfaces.md` first (the model, the invariants, the render boundaries) and
`app-data.md` (how to build `DATA` from the MCP tools).

## Why one object

Rendering appends a new widget every time — it never morphs in place. So every
network fetch would be a new box. The fix: **front-load the menu.** One `get_menu`
paints every category and product, and cart edits stay client-side — so browsing
and building the cart run with zero fetches, one seamless window. Options are
fetched **only on real intent** (a named customization up front, or lazily when the
user opens a browsed item's sheet), never speculatively pre-fetched for a whole
section — that would be an API call-burst Swiggy flags (see `surfaces.md`). A new
render happens only when you cross into a *different* restaurant, open a
not-yet-loaded customize sheet, or pull the real bill.

## What you inject: the `DATA` object

Replace `__DATA__` in the template with one JSON object of this shape. Omit what a
given entry doesn't need.

```js
{
  address: "Home · C V Raman Nagar",     // string, shown in headers
  entry: "menu",                          // "search" | "menu" | "customize" | "checkout"
  entryItemId: null,                      // required when entry === "customize" (must also be in items)
  entryCat: null,                         // optional: category tab to open on (with entry:"menu")

  restaurant: { id:"bk", name:"Burger King", rating:4.2, eta:"20–25 min" },

  // search screen only (entry:"search"): the ranked, ad-stripped list (app-data.md)
  restaurants: [ { id, name, rating, eta, tag } ],

  categories: ["Burgers","Sides","Drinks"],           // deduped, sectionized (app-data.md)
  items: {                                            // keyed by category
    "Burgers": [ { id:"owv", n:"Original Whopper Veg", p:189, veg:1, cz:true },
                 { id:"xx",  n:"Sold-out Special", p:99, veg:1, oos:true } ],  // oos → sold-out, not addable
    "Sides":   [ { id:"kf",  n:"King Fries", p:139, veg:1 } ]
  },

  // one entry per customizable item you pre-fetched options for; curated (app-data.md)
  customize: {
    "owv": { groups: [
      { key:"meal",   label:"make it a", type:"single", absolute:true,
        default:0, options:[ {id:"m1",n:"Burger only",price:189}, {id:"m2",n:"+ fries & coke",price:308} ] },
      { key:"cheese", label:"cheese", type:"single", default:0,
        options:[ {id:"c0",n:"none",price:0}, {id:"c1",n:"single",price:25} ] },
      { key:"addons", label:"add-ons", type:"multi", max:3,
        options:[ {id:"a1",n:"Peri peri mix",price:29}, {id:"a2",n:"Choco mousse",price:129} ] }
    ] }
  },

  cart: [ { key:"cv", itemId:"cv", n:"Crispy Veg Burger", p:70, q:2 } ],  // optional pre-fill (preset)
  cartRestaurantId: "bk",                 // restaurant the live backend cart belongs to (conflict guard)
  minOrder: null                          // optional: min order value (Instamart ₹99) — gates the cart bar
}
```

Shape rules the template relies on:
- A `single` group renders as a segmented control (exactly one selected). Mark the
  meal/base selector `absolute:true` — its chosen option price *replaces* the item
  base; every other group's prices *add*.
- A `multi` group renders as chips capped at `max`.
- An option with `price:0` shows no price (it reads as "included").
- An item with `cz:true` and no matching `customize[id]` entry still shows a customize
  button; tapping it hands back for one lazy `get_item_options` fetch (a new render).
  Provide a config only for an item fetched on intent — never pre-fetch a whole
  section (`app-data.md` §4).
- `entry:"customize"` requires the full menu in `categories`/`items` **and**
  `entryItemId` present in `items` — the customize view reads the item from `items`
  and its back button returns to the menu; an empty menu throws.
- `oos:true` on an item renders it sold-out (dimmed, not addable). Drop sold-out
  option choices from a `customize` config too.
- `minOrder` gates the cart bar: below it, the bar shows "add ₹X more" instead of
  checkout. Hard enforcement is still server-side at `im_prepare_order`.
- `entryCat` (with `entry:"menu"`) opens the app on that category tab, not the first.

## The hand-backs (the only messages the app sends)

Every price the app shows is an estimate (`≈`) — invariant 2. The app never
places and never invents the bill. It hands back via `sendPrompt` at exactly
these points:

- **open a restaurant** (search screen): `"Open {name} — show the menu."` You
  fetch its menu + options and render a fresh app with `entry:"menu"`.
- **customize an item with no pre-loaded config**: `"Customize {item} from
  {restaurant} — pull its options."` One lazy fetch, on tap (never pre-fetched).
- **checkout** (the money boundary): `"Pull the real bill for my {restaurant}
  cart and show it to confirm: {lines}."` You sync the cart (`update_cart`) then
  call `prepare_order` and render the bill-confirm surface
  (`checkout-and-edges.md`) — the only surface with a real total and the place button.
- **conflict keep**: `"Keep my current cart — cancel that add."`
- **conflict switch**: `"Switch to {restaurant} — clear my other cart and start fresh with {item}."`

## The template — copy verbatim, replace `__DATA__`

```html
<h2 class="sr-only">Ordering app: browse a restaurant menu by category, add or customize items, and check out — all in one window.</h2>
<div style="max-width:450px"><div id="app" style="min-height:400px"></div></div>
<script>
const DATA=__DATA__;
const A=document.getElementById("app");
let rid=DATA.restaurant?DATA.restaurant.id:null, cat=DATA.entryCat||null, cz=null, sq="",
    cart=(DATA.cart||[]).slice(), cartRid=DATA.cartRestaurantId||null, pend=null, czState=null;
const money=n=>"₹"+Math.round(n);
const esc=s=>String(s==null?"":s).replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/"/g,"&quot;").replace(/'/g,"&#39;");
const veg=v=>{const c=v?"var(--text-success)":"var(--text-danger)";return `<span style="width:14px;height:14px;border-radius:3px;border:1.5px solid ${c};display:inline-flex;align-items:center;justify-content:center;flex:none"><span style="width:5px;height:5px;border-radius:50%;background:${c}"></span></span>`};
const itemById=id=>{for(const c in DATA.items)for(const it of DATA.items[c])if(it.id===id)return it;return null;};
const cnt=()=>cart.reduce((a,l)=>a+l.q,0);
const tot=()=>cart.reduce((a,l)=>a+l.p*l.q,0);
const chip=(on,label,extra)=>`cursor:pointer;font-size:12px;padding:${extra||"6px 11px"};border-radius:999px;border:0.5px solid ${on?"var(--border-accent)":"var(--border-strong)"};background:${on?"var(--bg-accent)":"transparent"};color:${on?"var(--text-accent)":"inherit"}`;
const cta="width:100%;height:42px;border-color:var(--border-accent);color:var(--text-accent);background:var(--bg-accent)";
function bar(){const n=cnt();if(!n)return "";const under=DATA.minOrder&&tot()<DATA.minOrder;const right=under?`<span style="font-size:13px;color:var(--text-warning)">add ${money(DATA.minOrder-tot())} more</span>`:`<button onclick="GO('cart')" style="border-color:var(--border-accent);color:var(--text-accent);background:var(--bg-accent);padding:6px 13px">checkout →</button>`;return `<div style="position:sticky;bottom:0;background:var(--surface-1);border-radius:12px;padding:10px 14px;margin-top:8px;display:flex;justify-content:space-between;align-items:center"><span style="font-size:14px">${n} in cart · ≈ ${money(tot())}</span>${right}</div>`}
function header(title,sub,back){return `<div style="display:flex;align-items:center;gap:10px;margin-bottom:10px">${back?`<button onclick="${back}" style="padding:4px 9px;font-size:13px;flex:none"><i class="ti ti-arrow-left" style="font-size:13px;vertical-align:-2px"></i></button>`:""}<div style="flex:1;min-width:0"><div style="font-size:15px;font-weight:500">${title}</div><div style="font-size:12px;color:var(--text-secondary)">${sub}</div></div></div>`}
function search(){const q=sq.toLowerCase();const list=(DATA.restaurants||[]).filter(r=>!q||r.name.toLowerCase().includes(q)||(r.tag||"").includes(q));
 const rows=list.map(r=>`<div onclick="OPEN('${r.id}')" style="display:flex;align-items:center;gap:12px;padding:12px 0;border-top:0.5px solid var(--border);cursor:pointer"><div style="flex:1;min-width:0"><div style="font-size:14px;font-weight:500">${esc(r.name)}</div><div style="font-size:12px;color:var(--text-secondary);margin-top:2px"><i class="ti ti-clock" style="font-size:11px;vertical-align:-1px"></i> ${esc(r.eta)}${r.tag?" · "+esc(r.tag):""}</div></div><span style="font-size:12px;padding:2px 7px;border-radius:999px;background:var(--bg-success);color:var(--text-success)"><i class="ti ti-star" style="font-size:11px;vertical-align:-1px"></i> ${r.rating}</span><i class="ti ti-chevron-right" style="font-size:16px;color:var(--text-muted)"></i></div>`).join("");
 A.innerHTML=`<div style="display:flex;align-items:center;gap:8px;background:var(--surface-2);border:0.5px solid var(--border);border-radius:var(--radius);padding:0 10px;margin-bottom:6px"><i class="ti ti-search" style="font-size:16px;color:var(--text-muted)"></i><input id="sb" value="${esc(sq)}" placeholder="search restaurants or a dish" oninput="SEARCH(this.value)" style="border:none;background:transparent;flex:1;height:38px;outline:none;font-size:14px;color:var(--text-primary)"></div><div style="font-size:12px;color:var(--text-secondary);margin:8px 0 2px">${q?"ranked by rating":"top rated near you"}</div>${rows||'<div style="padding:16px 0;color:var(--text-muted);font-size:13px">nothing matches</div>'}${bar()}`;}
function menu(){const back=DATA.restaurants?"GO('search')":null;const cats=DATA.categories;if(!cat||!DATA.items[cat])cat=cats[0];
 const tabs=cats.map(c=>`<button onclick="CAT('${c}')" style="${chip(c===cat,c)};white-space:nowrap">${c}</button>`).join("");
 const rows=DATA.items[cat].map(it=>{const l=cart.find(x=>x.key===it.id);const q=l?l.q:0;
  const ctrl=it.oos?`<span style="font-size:12px;color:var(--text-danger);flex:none">sold out</span>`:it.cz?`<button onclick="CZ('${it.id}')" style="padding:5px 12px;flex:none">customize</button>`:q>0?`<div style="display:flex;align-items:center;gap:9px;flex:none"><button onclick="DEC('${it.id}')" style="padding:2px 9px">−</button><span style="font-weight:500;min-width:12px;text-align:center">${q}</span><button onclick="ADD('${it.id}')" style="padding:2px 9px">+</button></div>`:`<button onclick="ADD('${it.id}')" style="padding:5px 14px;flex:none">add</button>`;
  return `<div style="display:flex;align-items:center;gap:10px;padding:11px 0;border-top:0.5px solid var(--border)${it.oos?";opacity:.5":""}">${veg(it.veg)}<div style="flex:1;min-width:0"><div style="font-size:14px;font-weight:${q>0?500:400}">${esc(it.n)}</div><div style="font-size:13px;color:var(--text-secondary)">${money(it.p)}${it.cz&&!it.oos?" · customizable":""}</div></div>${ctrl}</div>`}).join("");
 A.innerHTML=header(DATA.restaurant.name,DATA.address,back)+`<div style="display:flex;gap:7px;overflow-x:auto;padding-bottom:8px">${tabs}</div>${rows}${bar()}`;}
function czView(){const it=itemById(cz),cfg=DATA.customize[cz];if(!cfg){SEND(`Customize ${it.n} from ${DATA.restaurant.name} — pull its options.`);return;}
 if(!czState){czState={};cfg.groups.forEach(g=>czState[g.key]=g.type==="multi"?new Set():(g.default||0));}
 let base=it.p,add=0;cfg.groups.forEach(g=>{if(g.type==="single"){const o=g.options[czState[g.key]];if(g.absolute)base=o.price;else add+=o.price;}else[...czState[g.key]].forEach(i=>add+=g.options[i].price);});const price=base+add;czState._price=price;
 const grp=g=>{if(g.type==="single")return `<div style="font-size:13px;color:var(--text-secondary);margin-top:12px">${g.label}</div><div style="display:flex;gap:6px;flex-wrap:wrap;margin:6px 0 2px">${g.options.map((o,i)=>`<button onclick="PICK('${g.key}',${i})" style="${chip(czState[g.key]===i,"","6px 11px")};border-radius:var(--radius)">${o.n}${o.price?" "+money(o.price):""}</button>`).join("")}</div>`;
  return `<div style="font-size:13px;color:var(--text-secondary);margin-top:12px">${g.label}${g.max>1?" · up to "+g.max:""}</div><div style="display:flex;gap:6px;flex-wrap:wrap;margin:6px 0 2px">${g.options.map((o,i)=>`<button onclick="TOG('${g.key}',${i},${g.max})" style="${chip(czState[g.key].has(i),"","5px 10px")}">${o.n}${o.price?" +"+money(o.price):""}</button>`).join("")}</div>`};
 A.innerHTML=`<button onclick="GO('menu')" style="padding:4px 10px;font-size:13px;margin-bottom:10px"><i class="ti ti-arrow-left" style="font-size:13px;vertical-align:-2px"></i> ${DATA.restaurant.name}</button><div style="display:flex;align-items:center;gap:8px">${veg(it.veg)}<span style="font-size:16px;font-weight:500">${it.n}</span></div><div style="font-size:12px;color:var(--text-secondary)">customize — in this window</div>${cfg.groups.map(grp).join("")}<button onclick="ADDCZ()" style="${cta};margin-top:16px"><i class="ti ti-plus" style="font-size:15px;vertical-align:-2px"></i> add to cart · ≈ ${money(price)}</button>`;}
function cartView(){if(!cnt()){A.innerHTML=header("Cart",DATA.address,"GO('menu')")+`<div style="padding:24px 0;text-align:center;color:var(--text-muted);font-size:14px">cart empty</div>`;return;}
 const rows=cart.map(l=>`<div style="display:flex;justify-content:space-between;font-size:14px;padding:5px 0"><span>${l.q} × ${l.n}</span><span>≈ ${money(l.p*l.q)}</span></div>`).join("");
 A.innerHTML=header("Checkout · "+(DATA.restaurant?DATA.restaurant.name:""),DATA.address,"GO('menu')")+rows+`<div style="display:flex;justify-content:space-between;font-weight:500;border-top:0.5px solid var(--border);margin-top:8px;padding-top:8px"><span>estimate</span><span>≈ ${money(tot())}</span></div><div style="font-size:12px;color:var(--text-muted);margin:6px 0 14px">delivery, taxes & offers finalize when you place</div><button onclick="CHECKOUT()" style="${cta};height:44px"><i class="ti ti-lock" style="font-size:15px;vertical-align:-2px"></i> review &amp; place ↗</button>`;}
function conflict(){const it=itemById(pend);A.innerHTML=`<div style="max-width:400px;background:var(--surface-2);border:0.5px solid var(--border);border-radius:12px;padding:16px 18px"><div style="display:flex;gap:10px;align-items:flex-start;margin-bottom:8px"><i class="ti ti-alert-triangle" style="font-size:20px;color:var(--text-warning);flex:none;margin-top:2px"></i><div><div style="font-size:15px;font-weight:500">Different restaurant</div><div style="font-size:13px;color:var(--text-secondary);margin-top:3px">Your cart is from another restaurant. Adding ${it.n} from ${DATA.restaurant.name} starts a fresh cart.</div></div></div><div style="display:flex;gap:8px;margin-top:14px"><button onclick="KEEP()" style="flex:1">keep current</button><button onclick="SWITCH()" style="flex:1;border-color:var(--border-accent);color:var(--text-accent);background:var(--bg-accent)">switch</button></div></div>`;}
function conflictGuard(id){if(cartRid&&cartRid!==rid&&cnt()){pend=id;conflict();return true;}return false;}
window.SEARCH=v=>{sq=v;search();const b=document.getElementById("sb");b.focus();b.setSelectionRange(b.value.length,b.value.length);};
window.OPEN=id=>{const r=(DATA.restaurants||[]).find(x=>x.id===id);SEND(`Open ${r?r.name:""} — show the menu.`);};
window.CAT=c=>{cat=c;menu();};
window.GO=v=>{if(v==="search")search();else if(v==="menu")menu();else if(v==="cart")cartView();};
window.ADD=id=>{if(conflictGuard(id))return;const it=itemById(id);const l=cart.find(x=>x.key===id);if(l)l.q++;else cart.push({key:id,itemId:id,n:it.n,p:it.p,q:1});cartRid=rid;menu();};
window.DEC=id=>{const l=cart.find(x=>x.key===id);if(l){l.q--;if(l.q<=0)cart=cart.filter(x=>x.key!==id);}if(!cnt())cartRid=null;menu();};
window.CZ=id=>{if(conflictGuard(id))return;cz=id;czState=null;czView();};
window.PICK=(k,i)=>{czState[k]=i;czView();};
window.TOG=(k,i,max)=>{const s=czState[k];if(s.has(i))s.delete(i);else if(s.size<max)s.add(i);czView();};
window.ADDCZ=()=>{const it=itemById(cz),cfg=DATA.customize[cz];const bits=[];cfg.groups.forEach(g=>{if(g.type==="single"){const o=g.options[czState[g.key]];if(o.price||g.absolute)bits.push(o.n);}else[...czState[g.key]].forEach(i=>bits.push(g.options[i].n));});const label=it.n+(bits.length?" ("+bits.join(", ")+")":"");const key=cz+"|"+bits.join(",");const l=cart.find(x=>x.key===key);if(l)l.q++;else cart.push({key,itemId:cz,n:label,p:czState._price,q:1});cartRid=rid;menu();};
window.KEEP=()=>{pend=null;SEND(`Keep my current cart — cancel that add.`);};
window.SWITCH=()=>{const it=itemById(pend);cart=[{key:pend,itemId:pend,n:it.n,p:it.p,q:1}];cartRid=rid;pend=null;SEND(`Switch to ${DATA.restaurant.name} — clear my other cart and start fresh with ${it.n}.`);};
window.CHECKOUT=()=>{const lines=cart.map(l=>`${l.q}× ${l.n}`).join("; ");SEND(`Pull the real bill for my ${DATA.restaurant?DATA.restaurant.name:""} cart and show it to confirm: ${lines}.`);};
window.SEND=t=>{try{sendPrompt(t);}catch(e){}};
(function(){const e=DATA.entry||"menu";if(e==="search")search();else if(e==="customize"){cz=DATA.entryItemId;czState=null;czView();}else if(e==="checkout")cartView();else menu();})();
</script>
```

## Notes

- The estimate is client-side and always marked `≈`. The authoritative total only
  ever comes from `prepare_order` on the bill-confirm surface. Never edit the
  template to show a computed total as the charge.
- Conflict here is the *UI*; the real backend guard still happens agent-side before
  `update_cart` (see `surfaces.md` invariant 3 and `checkout-and-edges.md`). The
  switch/keep buttons hand the decision back to you to execute.
- Keep the template a single unit. If you must adjust data, adjust `DATA` — do not
  fork the markup per restaurant.
