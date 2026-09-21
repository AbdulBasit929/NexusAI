import{a as e}from"./chunk-Cyuzqnbw.js";import{n as t,t as n}from"./jsx-runtime-xIJTzhZo.js";import{r}from"./api-BkO66t48.js";import{D as i,O as a,k as o,t as s}from"./index-CqWqHnpN.js";import{t as c}from"./ConfirmDialog-D3MvG0t4.js";import{t as ee}from"./PageHeader-CiIbNhP-.js";var l=e(t(),1),u=n();function d(){let{name:e}=a(),{addToast:t}=i(),[n]=o(),d=n.get(`user_id`)||void 0,{caseOptions:te,registryState:f,ensureCaseRegistry:p,setActiveCase:m}=s(),h=d?null:te.find(t=>t.collectionId===e),[g,_]=(0,l.useState)(`entries`),[v,y]=(0,l.useState)(!0),[b,x]=(0,l.useState)(null),[S,C]=(0,l.useState)([]),[w,T]=(0,l.useState)(null),[E,D]=(0,l.useState)(!1),[O,ne]=(0,l.useState)(null),[k,re]=(0,l.useState)(``),[A,j]=(0,l.useState)(10),[M,N]=(0,l.useState)([]),[P,F]=(0,l.useState)(null),[I,L]=(0,l.useState)(!1),[R,z]=(0,l.useState)(null),[B,V]=(0,l.useState)(null),[H,U]=(0,l.useState)(!1),[W,G]=(0,l.useState)([]),[K,q]=(0,l.useState)(``),[J,Y]=(0,l.useState)(``),[X,Z]=(0,l.useState)(!1),Q=(0,l.useCallback)(async()=>{try{let t=await r.entries(e,d);C(Array.isArray(t.entries)?t.entries:[])}catch(e){t(`Failed to load entries: ${e.message}`,`error`)}},[e,t,d]),$=(0,l.useCallback)(async()=>{try{let t=await r.sources(e,d);G(Array.isArray(t.sources)?t.sources:[])}catch(e){t(`Failed to load sources: ${e.message}`,`error`)}},[e,t,d]);(0,l.useEffect)(()=>{(async()=>{y(!0),await Promise.allSettled([Q(),$()]),y(!1)})()},[Q,$]),(0,l.useEffect)(()=>{!d&&f===`idle`&&p()},[p,f,d]);let ie=async n=>{z(n),V(null),U(!0);try{V(await r.entryContent(e,n,d))}catch(i){try{V(await r.rawEntryContent(e,n,d))}catch(e){t(`Failed to load entry content: ${e.message||i.message}`,`error`),z(null)}}finally{U(!1)}},ae=async n=>{x({title:`Delete Entry`,message:`Are you sure you want to delete this entry?`,confirmLabel:`Delete`,danger:!0,onConfirm:async()=>{x(null);try{await r.deleteEntry(e,n,d),t(`Entry deleted`,`success`),Q()}catch(e){t(`Failed to delete entry: ${e.message}`,`error`)}}})},oe=async n=>{if(n.preventDefault(),w){D(!0);try{let n=new FormData;n.append(`file`,w);let i=await r.upload(e,n,d);ne(i),i?.records_status===`queued`?t(`File uploaded and structured records ingest queued`,`success`):i?.records_status===`failed`?t(`File uploaded to KB, but records ingest forwarding failed`,`warning`):t(`File uploaded successfully`,`success`),T(null),Q()}catch(e){t(`Upload failed: ${e.message}`,`error`)}finally{D(!1)}}},se=async n=>{if(n.preventDefault(),k.trim()){L(!0);try{let t=await r.search(e,k,A,d);N(Array.isArray(t.results)?t.results:[]),F({count:t.count,requestedMaxResults:t.requested_max_results,effectiveMaxResults:t.effective_max_results})}catch(e){t(`Search failed: ${e.message}`,`error`)}finally{L(!1)}}},ce=async n=>{if(n.preventDefault(),K.trim()){Z(!0);try{await r.addSource(e,K,J||void 0,d),t(`Source added`,`success`),q(``),Y(``),$()}catch(e){t(`Failed to add source: ${e.message}`,`error`)}finally{Z(!1)}}},le=async n=>{x({title:`Remove Source`,message:`Are you sure you want to remove this source?`,confirmLabel:`Remove`,danger:!0,onConfirm:async()=>{x(null);try{await r.removeSource(e,n,d),t(`Source removed`,`success`),$()}catch(e){t(`Failed to remove source: ${e.message}`,`error`)}}})};return(0,u.jsxs)(`div`,{className:`page page--narrow`,children:[(0,u.jsx)(`style`,{children:`
        .collection-detail-upload-form {
          display: flex;
          align-items: center;
          gap: var(--spacing-sm);
          margin-bottom: var(--spacing-md);
          flex-wrap: wrap;
        }
        .collection-detail-upload-status {
          border: 1px solid var(--color-border);
          border-radius: var(--radius-md);
          padding: var(--spacing-md);
          margin-bottom: var(--spacing-md);
          background: var(--color-bg-secondary);
        }
        .collection-detail-upload-status-title {
          display: flex;
          align-items: center;
          gap: var(--spacing-xs);
          margin: 0 0 var(--spacing-xs);
          font-size: 0.9375rem;
          font-weight: 600;
          color: var(--color-text-primary);
        }
        .collection-detail-upload-status-grid {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
          gap: var(--spacing-sm);
          margin-top: var(--spacing-sm);
        }
        .collection-detail-upload-status-item {
          min-width: 0;
          font-size: 0.8125rem;
          color: var(--color-text-secondary);
        }
        .collection-detail-upload-status-item strong {
          display: block;
          margin-top: 2px;
          color: var(--color-text-primary);
          overflow-wrap: anywhere;
        }
        .collection-detail-upload-status-warning {
          margin-top: var(--spacing-sm);
          padding: var(--spacing-sm);
          border-radius: var(--radius-sm);
          background: rgba(245, 158, 11, 0.12);
          color: var(--color-warning, #92400e);
          font-size: 0.8125rem;
          overflow-wrap: anywhere;
        }
        .collection-detail-search-form {
          display: flex;
          align-items: flex-end;
          gap: var(--spacing-sm);
          margin-bottom: var(--spacing-md);
          flex-wrap: wrap;
        }
        .collection-detail-search-form .collection-detail-field {
          display: flex;
          flex-direction: column;
          gap: var(--spacing-xs);
        }
        .collection-detail-search-form .collection-detail-field label {
          font-size: 0.8125rem;
          font-weight: 500;
          color: var(--color-text-secondary);
        }
        .collection-detail-result-card {
          background: var(--color-bg-secondary);
          border-radius: var(--radius-md);
          padding: var(--spacing-md);
          margin-bottom: var(--spacing-sm);
        }
        .collection-detail-result-score {
          font-size: 0.75rem;
          font-weight: 600;
          color: var(--color-primary);
          margin-bottom: var(--spacing-xs);
        }
        .collection-detail-search-meta {
          display: flex;
          flex-wrap: wrap;
          gap: var(--spacing-xs);
          margin-bottom: var(--spacing-md);
          color: var(--color-text-secondary);
          font-size: 0.8125rem;
        }
        .collection-detail-search-meta span {
          border: 1px solid var(--color-border);
          border-radius: var(--radius-sm);
          padding: 4px 8px;
          background: var(--color-bg-secondary);
        }
        .collection-detail-result-content {
          font-size: 0.875rem;
          color: var(--color-text-primary);
          white-space: pre-wrap;
          word-break: break-word;
        }
        .collection-detail-source-form {
          display: flex;
          align-items: flex-end;
          gap: var(--spacing-sm);
          margin-bottom: var(--spacing-md);
          flex-wrap: wrap;
        }
        .collection-detail-source-form .collection-detail-field {
          display: flex;
          flex-direction: column;
          gap: var(--spacing-xs);
          flex: 1;
          min-width: 200px;
        }
        .collection-detail-source-form .collection-detail-field label {
          font-size: 0.8125rem;
          font-weight: 500;
          color: var(--color-text-secondary);
        }
        .collection-detail-entry-content {
          max-width: 400px;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
          font-size: 0.8125rem;
          color: var(--color-text-secondary);
        }
        .collection-detail-empty {
          text-align: center;
          padding: var(--spacing-xl);
          color: var(--color-text-muted);
        }
        .collection-detail-modal-overlay {
          position: fixed;
          inset: 0;
          background: rgba(0, 0, 0, 0.5);
          display: flex;
          align-items: center;
          justify-content: center;
          z-index: 1000;
        }
        .collection-detail-modal {
          background: var(--color-bg-primary);
          border-radius: var(--radius-lg);
          width: 90%;
          max-width: 700px;
          max-height: 80vh;
          display: flex;
          flex-direction: column;
          box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
        }
        .collection-detail-modal-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: var(--spacing-md) var(--spacing-lg);
          border-bottom: 1px solid var(--color-border);
        }
        .collection-detail-modal-header h3 {
          margin: 0;
          font-size: 1rem;
          font-weight: 600;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
        .collection-detail-modal-body {
          padding: var(--spacing-lg);
          overflow-y: auto;
          flex: 1;
        }
        .collection-detail-modal-content {
          white-space: pre-wrap;
          word-break: break-word;
          font-family: var(--font-mono);
          font-size: 0.8125rem;
          background: var(--color-bg-secondary);
          border-radius: var(--radius-md);
          padding: var(--spacing-md);
          max-height: 50vh;
          overflow-y: auto;
        }
      `}),(0,u.jsx)(ee,{title:e,supporting:`Administrative collection details, retrieval, sources, and lifecycle management.`,actions:h?(0,u.jsxs)(`button`,{className:`btn btn-primary`,type:`button`,onClick:()=>m(h.caseId,`overview`),children:[(0,u.jsx)(`i`,{className:`fas fa-shield-halved`}),` Open authorized case`]}):null}),(0,u.jsxs)(`div`,{className:`alert alert-info`,role:`note`,children:[(0,u.jsx)(`i`,{className:`fas fa-circle-info`}),` `,h?`Authorized case mapping verified: ${h.displayName} (${h.caseId}).`:`Administrative collection only. No authorized selectable case mapping is available for this collection.`]}),(0,u.jsxs)(`div`,{className:`tabs`,children:[(0,u.jsxs)(`button`,{className:`tab ${g===`entries`?`tab-active`:``}`,onClick:()=>_(`entries`),children:[(0,u.jsx)(`i`,{className:`fas fa-list`}),` Entries`]}),(0,u.jsxs)(`button`,{className:`tab ${g===`search`?`tab-active`:``}`,onClick:()=>_(`search`),children:[(0,u.jsx)(`i`,{className:`fas fa-search`}),` Search`]}),(0,u.jsxs)(`button`,{className:`tab ${g===`sources`?`tab-active`:``}`,onClick:()=>_(`sources`),children:[(0,u.jsx)(`i`,{className:`fas fa-globe`}),` Sources`]})]}),(0,u.jsxs)(`p`,{style:{color:`var(--color-text-secondary)`,fontSize:`0.875rem`,marginTop:0},children:[`Collection search retrieves evidence and raw previews. `,h?(0,u.jsxs)(u.Fragment,{children:[`Use the explicit `,(0,u.jsx)(`button`,{className:`btn-link`,type:`button`,onClick:()=>m(h.caseId,`analyze`),children:`authorized case analysis`}),` for exact counts, filters, rankings, min/max, and joins over structured files.`]}):(0,u.jsx)(u.Fragment,{children:`Exact records analysis requires an authorized case mapping.`})]}),v?(0,u.jsx)(`div`,{style:{display:`flex`,justifyContent:`center`,padding:`var(--spacing-xl)`},children:(0,u.jsx)(`i`,{className:`fas fa-spinner fa-spin`,style:{fontSize:`2rem`,color:`var(--color-primary)`}})}):g===`entries`?(0,u.jsxs)(u.Fragment,{children:[(0,u.jsxs)(`form`,{className:`collection-detail-upload-form`,onSubmit:oe,children:[(0,u.jsx)(`input`,{className:`input`,type:`file`,onChange:e=>T(e.target.files[0]||null),style:{flex:1,minWidth:200}}),(0,u.jsx)(`button`,{className:`btn btn-primary`,type:`submit`,disabled:!w||E,children:E?(0,u.jsxs)(u.Fragment,{children:[(0,u.jsx)(`i`,{className:`fas fa-spinner fa-spin`}),` Uploading...`]}):(0,u.jsxs)(u.Fragment,{children:[(0,u.jsx)(`i`,{className:`fas fa-upload`}),` Upload`]})})]}),O&&(0,u.jsxs)(`div`,{className:`collection-detail-upload-status`,role:`status`,children:[(0,u.jsxs)(`p`,{className:`collection-detail-upload-status-title`,children:[(0,u.jsx)(`i`,{className:O.records_status===`failed`?`fas fa-triangle-exclamation`:`fas fa-circle-check`}),`Upload pipeline status`]}),(0,u.jsxs)(`div`,{className:`collection-detail-upload-status-grid`,children:[(0,u.jsxs)(`div`,{className:`collection-detail-upload-status-item`,children:[`KB entry`,(0,u.jsx)(`strong`,{children:O.key||`stored`})]}),(0,u.jsxs)(`div`,{className:`collection-detail-upload-status-item`,children:[`File`,(0,u.jsx)(`strong`,{children:O.filename||w?.name||`-`})]}),(0,u.jsxs)(`div`,{className:`collection-detail-upload-status-item`,children:[`Records pipeline`,(0,u.jsx)(`strong`,{children:O.records_status||`not forwarded`})]}),O.records_ingest?.job_id&&(0,u.jsxs)(`div`,{className:`collection-detail-upload-status-item`,children:[`Records job`,(0,u.jsx)(`strong`,{children:O.records_ingest.job_id})]}),O.records_ingest?.detected_record_type&&(0,u.jsxs)(`div`,{className:`collection-detail-upload-status-item`,children:[`Detected type`,(0,u.jsx)(`strong`,{children:O.records_ingest.detected_record_type})]})]}),O.records_warning&&(0,u.jsx)(`div`,{className:`collection-detail-upload-status-warning`,children:O.records_warning})]}),S.length===0?(0,u.jsxs)(`div`,{className:`collection-detail-empty`,children:[(0,u.jsx)(`i`,{className:`fas fa-inbox`,style:{fontSize:`2rem`,marginBottom:`var(--spacing-sm)`,display:`block`}}),(0,u.jsx)(`p`,{children:`No entries in this collection. Upload a file to get started.`})]}):(0,u.jsx)(`div`,{className:`table-container`,children:(0,u.jsxs)(`table`,{className:`table`,children:[(0,u.jsx)(`thead`,{children:(0,u.jsxs)(`tr`,{children:[(0,u.jsx)(`th`,{children:`Entry`}),(0,u.jsx)(`th`,{style:{textAlign:`right`},children:`Actions`})]})}),(0,u.jsx)(`tbody`,{children:S.map((e,t)=>(0,u.jsxs)(`tr`,{children:[(0,u.jsx)(`td`,{children:(0,u.jsx)(`div`,{className:`collection-detail-entry-content`,children:typeof e==`string`?e:JSON.stringify(e)})}),(0,u.jsx)(`td`,{children:(0,u.jsxs)(`div`,{style:{display:`flex`,justifyContent:`flex-end`,gap:`var(--spacing-xs)`},children:[(0,u.jsx)(`button`,{className:`btn btn-secondary btn-sm`,onClick:()=>ie(e),title:`View Content`,children:(0,u.jsx)(`i`,{className:`fas fa-eye`})}),(0,u.jsx)(`button`,{className:`btn btn-danger btn-sm`,onClick:()=>ae(e),title:`Delete`,children:(0,u.jsx)(`i`,{className:`fas fa-trash`})})]})})]},t))})]})})]}):g===`search`?(0,u.jsxs)(u.Fragment,{children:[(0,u.jsxs)(`form`,{className:`collection-detail-search-form`,onSubmit:se,children:[(0,u.jsxs)(`div`,{className:`collection-detail-field`,style:{flex:2,minWidth:250},children:[(0,u.jsx)(`label`,{htmlFor:`search-query`,children:`Query`}),(0,u.jsx)(`input`,{id:`search-query`,className:`input`,type:`text`,value:k,onChange:e=>re(e.target.value),placeholder:`Enter search query...`})]}),(0,u.jsxs)(`div`,{className:`collection-detail-field`,style:{flex:0,minWidth:100},children:[(0,u.jsx)(`label`,{htmlFor:`search-max`,children:`Max Results`}),(0,u.jsx)(`input`,{id:`search-max`,className:`input`,type:`number`,min:1,max:100,value:A,onChange:e=>j(parseInt(e.target.value,10)||10),style:{width:100}})]}),(0,u.jsx)(`button`,{className:`btn btn-primary`,type:`submit`,disabled:!k.trim()||I,children:I?(0,u.jsxs)(u.Fragment,{children:[(0,u.jsx)(`i`,{className:`fas fa-spinner fa-spin`}),` Searching...`]}):(0,u.jsxs)(u.Fragment,{children:[(0,u.jsx)(`i`,{className:`fas fa-search`}),` Search`]})})]}),P&&(0,u.jsxs)(`div`,{className:`collection-detail-search-meta`,role:`status`,children:[(0,u.jsxs)(`span`,{children:[`Returned: `,P.count??M.length]}),(0,u.jsxs)(`span`,{children:[`Requested max: `,P.requestedMaxResults??A]}),(0,u.jsxs)(`span`,{children:[`Effective max: `,P.effectiveMaxResults??P.count??M.length]}),Number(P.effectiveMaxResults)<Number(P.requestedMaxResults)&&(0,u.jsx)(`span`,{children:`Auto-capped to available indexed chunks`})]}),M.length===0?(0,u.jsxs)(`div`,{className:`collection-detail-empty`,children:[(0,u.jsx)(`i`,{className:`fas fa-search`,style:{fontSize:`2rem`,marginBottom:`var(--spacing-sm)`,display:`block`}}),(0,u.jsx)(`p`,{children:`No results. Enter a query and click Search.`})]}):M.map((e,t)=>(0,u.jsxs)(`div`,{className:`collection-detail-result-card`,children:[(0,u.jsxs)(`div`,{className:`collection-detail-result-score`,children:[`Similarity: `,typeof e.similarity==`number`?e.similarity.toFixed(4):e.score==null?`N/A`:Number(e.score).toFixed(4)]}),(0,u.jsx)(`div`,{className:`collection-detail-result-content`,children:e.content||e.text||(typeof e==`string`?e:JSON.stringify(e))})]},t))]}):(0,u.jsxs)(u.Fragment,{children:[(0,u.jsxs)(`form`,{className:`collection-detail-source-form`,onSubmit:ce,children:[(0,u.jsxs)(`div`,{className:`collection-detail-field`,children:[(0,u.jsx)(`label`,{htmlFor:`source-url`,children:`URL`}),(0,u.jsx)(`input`,{id:`source-url`,className:`input`,type:`text`,value:K,onChange:e=>q(e.target.value),placeholder:`https://example.com/data`})]}),(0,u.jsxs)(`div`,{className:`collection-detail-field`,style:{flex:0,minWidth:160},children:[(0,u.jsx)(`label`,{htmlFor:`source-interval`,children:`Update Interval`}),(0,u.jsx)(`input`,{id:`source-interval`,className:`input`,type:`text`,value:J,onChange:e=>Y(e.target.value),placeholder:`e.g. 1h, 30m`})]}),(0,u.jsx)(`button`,{className:`btn btn-primary`,type:`submit`,disabled:!K.trim()||X,children:X?(0,u.jsxs)(u.Fragment,{children:[(0,u.jsx)(`i`,{className:`fas fa-spinner fa-spin`}),` Adding...`]}):(0,u.jsxs)(u.Fragment,{children:[(0,u.jsx)(`i`,{className:`fas fa-plus`}),` Add Source`]})})]}),W.length===0?(0,u.jsxs)(`div`,{className:`collection-detail-empty`,children:[(0,u.jsx)(`i`,{className:`fas fa-globe`,style:{fontSize:`2rem`,marginBottom:`var(--spacing-sm)`,display:`block`}}),(0,u.jsx)(`p`,{children:`No external sources configured. Add a URL to start ingesting data.`})]}):(0,u.jsx)(`div`,{className:`table-container`,children:(0,u.jsxs)(`table`,{className:`table`,children:[(0,u.jsx)(`thead`,{children:(0,u.jsxs)(`tr`,{children:[(0,u.jsx)(`th`,{children:`URL`}),(0,u.jsx)(`th`,{children:`Interval`}),(0,u.jsx)(`th`,{style:{textAlign:`right`},children:`Actions`})]})}),(0,u.jsx)(`tbody`,{children:W.map((e,t)=>(0,u.jsxs)(`tr`,{children:[(0,u.jsx)(`td`,{style:{fontSize:`0.8125rem`,wordBreak:`break-all`},children:typeof e==`string`?e:e.url||JSON.stringify(e)}),(0,u.jsx)(`td`,{style:{fontSize:`0.8125rem`,color:`var(--color-text-secondary)`},children:typeof e==`object`&&e.update_interval?e.update_interval:`-`}),(0,u.jsx)(`td`,{children:(0,u.jsx)(`div`,{style:{display:`flex`,justifyContent:`flex-end`},children:(0,u.jsx)(`button`,{className:`btn btn-danger btn-sm`,onClick:()=>le(typeof e==`string`?e:e.url),title:`Remove`,children:(0,u.jsx)(`i`,{className:`fas fa-trash`})})})})]},t))})]})})]}),(0,u.jsx)(c,{open:!!b,title:b?.title,message:b?.message,confirmLabel:b?.confirmLabel,danger:b?.danger,onConfirm:b?.onConfirm,onCancel:()=>x(null)}),R&&(0,u.jsx)(`div`,{className:`collection-detail-modal-overlay`,onClick:()=>z(null),children:(0,u.jsxs)(`div`,{className:`collection-detail-modal`,onClick:e=>e.stopPropagation(),children:[(0,u.jsxs)(`div`,{className:`collection-detail-modal-header`,children:[(0,u.jsxs)(`h3`,{title:R,children:[(0,u.jsx)(`i`,{className:`fas fa-file-alt`,style:{marginRight:`var(--spacing-xs)`}}),R]}),(0,u.jsx)(`button`,{className:`btn btn-secondary btn-sm`,onClick:()=>z(null),children:(0,u.jsx)(`i`,{className:`fas fa-times`})})]}),(0,u.jsx)(`div`,{className:`collection-detail-modal-body`,children:H?(0,u.jsx)(`div`,{style:{textAlign:`center`,padding:`var(--spacing-lg)`},children:(0,u.jsx)(`i`,{className:`fas fa-spinner fa-spin`,style:{fontSize:`1.5rem`,color:`var(--color-primary)`}})}):B?(0,u.jsxs)(u.Fragment,{children:[(0,u.jsx)(`div`,{style:{display:`flex`,gap:`var(--spacing-md)`,marginBottom:`var(--spacing-md)`},children:(0,u.jsxs)(`div`,{style:{fontSize:`0.8125rem`,color:`var(--color-text-muted)`},children:[(0,u.jsx)(`i`,{className:`fas fa-puzzle-piece`,style:{marginRight:4}}),`Chunks: `,(0,u.jsx)(`strong`,{style:{color:`var(--color-text-primary)`},children:B.chunk_count??`-`})]})}),(0,u.jsx)(`div`,{className:`collection-detail-modal-content`,children:B.content||`(empty)`})]}):(0,u.jsx)(`p`,{style:{color:`var(--color-text-muted)`},children:`No content available.`})})]})})]})}export{d as default};