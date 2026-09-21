import{a as e}from"./chunk-Cyuzqnbw.js";import{n as t,t as n}from"./jsx-runtime-xIJTzhZo.js";import{I as r,a as i}from"./api-BkO66t48.js";import{D as a,E as o,O as s,k as c}from"./index-CqWqHnpN.js";import{t as l}from"./PageHeader-CiIbNhP-.js";var u=e(t(),1),d=n();function f({observable:e}){let t=e?.creation||{},n=e?.completion||{},r=``;if(t?.chat_completion_message?.content)r=t.chat_completion_message.content;else{let e=t?.chat_completion_request?.messages;Array.isArray(e)&&e.length>0&&(r=e[e.length-1]?.content||``)}typeof r==`object`&&(r=`Multimedia message`);let i=t?.function_definition?.name?`Function: ${t.function_definition.name}`:``,a=t?.function_params&&Object.keys(t.function_params).length>0?`Params: ${JSON.stringify(t.function_params)}`:``,o=``,s=``,c=n?.chat_completion_response;if(!c&&Array.isArray(n?.conversation)&&n.conversation.length>0&&(c={choices:n.conversation.map(e=>({message:e}))}),c?.choices?.length>0){let e=c.choices[c.choices.length-1],t=e?.message?.tool_calls;Array.isArray(t)&&t.length>0&&(s=t.map(e=>{let t=e.function?.arguments||``;return`${e.function?.name||`unknown`}(${typeof t==`string`?t:JSON.stringify(t)})`}).join(`, `)),o=e?.message?.content||``}let l=n?.action_result?String(n.action_result).slice(0,100):``,u=n?.error||``,f=``;if(n?.filter_result){let e=n.filter_result;e.has_triggers&&!e.triggered_by?f=`Failed to match triggers`:e.triggered_by&&(f=`Triggered by ${e.triggered_by}`),e.failed_by&&(f+=`${f?`, `:``}Failed by ${e.failed_by}`)}let p=[];return r&&p.push({icon:`fa-comment-dots`,text:r,cls:`creation`}),i&&p.push({icon:`fa-code`,text:i,cls:`creation`}),a&&p.push({icon:`fa-sliders-h`,text:a,cls:`creation`}),s&&p.push({icon:`fa-wrench`,text:s,cls:`tool-call`}),o&&p.push({icon:`fa-robot`,text:o,cls:`completion`}),l&&p.push({icon:`fa-bolt`,text:l,cls:`tool-call`}),u&&p.push({icon:`fa-exclamation-triangle`,text:u,cls:`error`}),f&&p.push({icon:`fa-shield-alt`,text:f,cls:`completion`}),p.length===0?null:(0,d.jsx)(`div`,{style:{display:`flex`,flexDirection:`column`,gap:2,marginTop:2},children:p.map((e,t)=>(0,d.jsxs)(`div`,{className:`as-summary-item as-summary-${e.cls}`,title:e.text,children:[(0,d.jsx)(`i`,{className:`fas ${e.icon}`}),(0,d.jsx)(`span`,{children:e.text})]},t))})}function p({observable:e,children:t}){let[n,r]=(0,u.useState)(!1),i=!!e.completion,a=e.progress?.length>0;return(0,d.jsxs)(`div`,{className:`as-card`,children:[(0,d.jsxs)(`div`,{className:`as-card-header`,onClick:()=>r(!n),children:[(0,d.jsxs)(`div`,{className:`as-card-title`,children:[(0,d.jsx)(`div`,{className:`as-obs-icon`,children:(0,d.jsx)(`i`,{className:`fas fa-${e.icon||`robot`}`})}),(0,d.jsxs)(`div`,{style:{flex:1,minWidth:0},children:[(0,d.jsxs)(`div`,{style:{display:`flex`,alignItems:`center`,gap:`var(--spacing-xs)`},children:[(0,d.jsx)(`span`,{style:{fontWeight:600,fontSize:`0.875rem`},children:e.name}),(0,d.jsxs)(`span`,{className:`as-id`,children:[`#`,e.id]}),!i&&(0,d.jsx)(`i`,{className:`fas fa-circle-notch fa-spin`,style:{fontSize:`0.7rem`,color:`var(--color-primary)`}})]}),(0,d.jsx)(f,{observable:e})]})]}),(0,d.jsx)(`i`,{className:`fas fa-chevron-${n?`up`:`down`}`,style:{color:`var(--color-text-muted)`,fontSize:`0.75rem`}})]}),n&&(0,d.jsxs)(`div`,{className:`as-card-body`,children:[t&&t.length>0&&(0,d.jsxs)(`div`,{style:{marginBottom:`var(--spacing-md)`},children:[(0,d.jsx)(`div`,{style:{fontSize:`0.75rem`,fontWeight:600,color:`var(--color-text-muted)`,textTransform:`uppercase`,marginBottom:`var(--spacing-xs)`},children:`Nested Observables`}),t]}),a&&(0,d.jsxs)(`div`,{style:{marginBottom:`var(--spacing-sm)`},children:[(0,d.jsxs)(`div`,{className:`as-section-label`,children:[`Progress (`,e.progress.length,`)`]}),e.progress.map((e,t)=>(0,d.jsxs)(`div`,{className:`as-progress-entry`,children:[e.action_result&&(0,d.jsxs)(`div`,{children:[(0,d.jsx)(`span`,{className:`as-tag`,children:`Action Result`}),` `,e.action_result]}),e.error&&(0,d.jsxs)(`div`,{className:`as-error-text`,children:[(0,d.jsx)(`span`,{className:`as-tag as-tag-error`,children:`Error`}),` `,e.error]}),e.chat_completion_response?.choices?.length>0&&(0,d.jsxs)(`div`,{children:[(0,d.jsx)(`span`,{className:`as-tag`,children:`Response`}),` `,e.chat_completion_response.choices.map((e,t)=>(0,d.jsx)(`span`,{children:e.message?.content||`(tool call)`},t))]}),e.agent_state&&(0,d.jsxs)(`div`,{children:[(0,d.jsx)(`span`,{className:`as-tag`,children:`State`}),` `,JSON.stringify(e.agent_state)]})]},t))]}),e.completion&&(0,d.jsxs)(`div`,{style:{marginBottom:`var(--spacing-sm)`},children:[(0,d.jsx)(`div`,{className:`as-section-label`,children:`Completion`}),e.completion.action_result&&(0,d.jsxs)(`div`,{className:`as-progress-entry`,children:[(0,d.jsx)(`span`,{className:`as-tag`,children:`Action Result`}),` `,e.completion.action_result]}),e.completion.error&&(0,d.jsxs)(`div`,{className:`as-progress-entry as-error-text`,children:[(0,d.jsx)(`span`,{className:`as-tag as-tag-error`,children:`Error`}),` `,e.completion.error]}),e.completion.filter_result&&(0,d.jsxs)(`div`,{className:`as-progress-entry`,children:[(0,d.jsx)(`span`,{className:`as-tag`,children:`Filter`}),` `,JSON.stringify(e.completion.filter_result)]})]}),(0,d.jsxs)(`details`,{className:`as-raw`,children:[(0,d.jsx)(`summary`,{children:`Raw JSON`}),(0,d.jsx)(`pre`,{className:`as-json`,children:JSON.stringify(e,null,2)})]})]})]})}function m(e){let t={};e.forEach(e=>{t[e.id]={...e,children:[]}});let n=[];return e.forEach(e=>{e.parent_id&&t[e.parent_id]?t[e.parent_id].children.push(t[e.id]):n.push(t[e.id])}),n}function h(e){return e.map(e=>(0,d.jsx)(p,{observable:e,children:e.children.length>0?h(e.children):null},e.id))}function g(){let{name:e}=s(),t=o(),{addToast:n}=a(),[f]=c(),p=f.get(`user_id`)||void 0,[g,_]=(0,u.useState)([]),[v,y]=(0,u.useState)(null),[b,x]=(0,u.useState)(!0),S=(0,u.useCallback)(async()=>{try{let t=await i.observables(e,p);_(Array.isArray(t)?t:t?.History||[])}catch(e){n(`Failed to load observables: ${e.message}`,`error`)}try{y(await i.status(e,p))}catch{}x(!1)},[e,p,n]);(0,u.useEffect)(()=>{S();let e=setInterval(S,5e3);return()=>clearInterval(e)},[S]),(0,u.useEffect)(()=>{let t=r(i.sseUrl(e,p)),n=new EventSource(t);return n.addEventListener(`observable_update`,e=>{try{let t=JSON.parse(e.data);_(e=>{let n=e.findIndex(e=>e.id===t.id);if(n>=0){let r=[...e],i=r[n];return r[n]={...i,...t,creation:t.creation||i.creation,completion:t.completion||i.completion,progress:(t.progress?.length??0)>(i.progress?.length??0)?t.progress:i.progress},r}return[...e,t]})}catch{}}),n.onerror=()=>{},()=>n.close()},[e,p]);let C=async()=>{try{await i.clearObservables(e,p),_([]),n(`Observables cleared`,`success`)}catch(e){n(`Failed to clear: ${e.message}`,`error`)}},w=m(g);return(0,d.jsxs)(`div`,{className:`page page--wide`,children:[(0,d.jsx)(`style`,{children:`
        .as-card {
          background: var(--color-bg-secondary);
          border: 1px solid var(--color-border);
          border-radius: var(--radius-md);
          margin-bottom: var(--spacing-sm);
          overflow: hidden;
        }
        .as-card .as-card {
          border-left: 3px solid var(--color-primary);
          margin-left: var(--spacing-md);
        }
        .as-card-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: 10px var(--spacing-md);
          cursor: pointer;
          gap: var(--spacing-sm);
        }
        .as-card-header:hover { background: var(--color-bg-tertiary); }
        .as-card-title { display: flex; align-items: flex-start; gap: var(--spacing-sm); flex: 1; min-width: 0; }
        .as-obs-icon {
          width: 28px; height: 28px;
          border-radius: var(--radius-md);
          background: var(--color-primary-light);
          color: var(--color-primary);
          display: flex; align-items: center; justify-content: center;
          font-size: 0.75rem; flex-shrink: 0;
        }
        .as-id {
          font-size: 0.6875rem;
          color: var(--color-text-muted);
          font-family: var(--font-mono);
        }
        .as-summary-item {
          display: flex; align-items: center; gap: 6px;
          font-size: 0.75rem; color: var(--color-text-secondary);
          overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
        }
        .as-summary-item i { font-size: 0.625rem; flex-shrink: 0; }
        .as-summary-creation i { color: var(--color-primary); }
        .as-summary-tool-call i { color: var(--color-warning); }
        .as-summary-completion i { color: var(--color-success); }
        .as-summary-error i { color: var(--color-error); }
        .as-card-body {
          padding: var(--spacing-md);
          border-top: 1px solid var(--color-border);
        }
        .as-section-label {
          font-size: 0.6875rem; font-weight: 600; text-transform: uppercase;
          letter-spacing: 0.04em; color: var(--color-text-muted);
          margin-bottom: var(--spacing-xs);
        }
        .as-progress-entry {
          font-size: 0.8125rem; color: var(--color-text-primary);
          padding: 4px 0; border-bottom: 1px solid var(--color-border-subtle);
          word-break: break-word;
        }
        .as-progress-entry:last-child { border-bottom: none; }
        .as-tag {
          display: inline-block; padding: 1px 6px; border-radius: var(--radius-sm);
          font-size: 0.625rem; font-weight: 600; text-transform: uppercase;
          background: var(--color-bg-tertiary); color: var(--color-text-muted);
          margin-right: 4px; vertical-align: middle;
        }
        .as-tag-error { background: var(--color-error); color: var(--color-text-inverse); }
        .as-error-text { color: var(--color-error); }
        .as-raw { margin-top: var(--spacing-sm); }
        .as-raw summary { font-size: 0.75rem; color: var(--color-text-muted); cursor: pointer; }
        .as-json {
          background: var(--color-bg-tertiary); border-radius: var(--radius-sm);
          padding: var(--spacing-sm); font-family: var(--font-mono);
          font-size: 0.75rem; overflow-x: auto; white-space: pre-wrap;
          word-break: break-word; max-height: 300px; overflow-y: auto;
        }
        .as-status-grid {
          display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
          gap: var(--spacing-sm); margin-bottom: var(--spacing-lg);
        }
        .as-status-item {
          background: var(--color-bg-secondary); border: 1px solid var(--color-border);
          border-radius: var(--radius-md); padding: var(--spacing-md);
        }
        .as-status-label {
          font-size: 0.6875rem; text-transform: uppercase; letter-spacing: 0.05em;
          color: var(--color-text-muted); margin-bottom: 4px;
        }
        .as-status-value { font-size: 1rem; font-weight: 600; color: var(--color-text-primary); }
      `}),(0,d.jsx)(l,{title:(0,d.jsxs)(d.Fragment,{children:[(0,d.jsx)(`i`,{className:`fas fa-chart-bar`,style:{marginRight:`var(--spacing-xs)`}}),e,` — Status`]}),supporting:`Agent observables and activity history`,actions:(0,d.jsxs)(`div`,{style:{display:`flex`,gap:`var(--spacing-sm)`},children:[(0,d.jsxs)(`button`,{className:`btn btn-secondary`,onClick:()=>t(`/app/agents/${encodeURIComponent(e)}/chat${p?`?user_id=${encodeURIComponent(p)}`:``}`),children:[(0,d.jsx)(`i`,{className:`fas fa-comment`}),` Chat`]}),(0,d.jsxs)(`button`,{className:`btn btn-secondary`,onClick:()=>t(`/app/agents/${encodeURIComponent(e)}/edit${p?`?user_id=${encodeURIComponent(p)}`:``}`),children:[(0,d.jsx)(`i`,{className:`fas fa-edit`}),` Edit`]}),(0,d.jsxs)(`button`,{className:`btn btn-secondary`,onClick:S,children:[(0,d.jsx)(`i`,{className:`fas fa-sync`}),` Refresh`]}),(0,d.jsxs)(`button`,{className:`btn btn-danger`,onClick:C,disabled:g.length===0,children:[(0,d.jsx)(`i`,{className:`fas fa-trash`}),` Clear`]})]})}),v&&(0,d.jsxs)(`div`,{className:`as-status-grid`,children:[v.state&&(0,d.jsxs)(`div`,{className:`as-status-item`,children:[(0,d.jsx)(`div`,{className:`as-status-label`,children:`State`}),(0,d.jsx)(`div`,{className:`as-status-value`,children:v.state})]}),v.current_task&&(0,d.jsxs)(`div`,{className:`as-status-item`,children:[(0,d.jsx)(`div`,{className:`as-status-label`,children:`Current Task`}),(0,d.jsx)(`div`,{className:`as-status-value`,style:{fontSize:`0.8125rem`,fontWeight:400},children:v.current_task})]}),(0,d.jsxs)(`div`,{className:`as-status-item`,children:[(0,d.jsx)(`div`,{className:`as-status-label`,children:`Observables`}),(0,d.jsx)(`div`,{className:`as-status-value`,children:g.length})]})]}),b?(0,d.jsx)(`div`,{style:{display:`flex`,justifyContent:`center`,padding:`var(--spacing-xl)`},children:(0,d.jsx)(`i`,{className:`fas fa-spinner fa-spin`,style:{fontSize:`2rem`,color:`var(--color-primary)`}})}):w.length===0?(0,d.jsxs)(`div`,{className:`empty-state`,children:[(0,d.jsx)(`div`,{className:`empty-state-icon`,children:(0,d.jsx)(`i`,{className:`fas fa-chart-bar`})}),(0,d.jsx)(`h2`,{className:`empty-state-title`,children:`No observables yet`}),(0,d.jsx)(`p`,{className:`empty-state-text`,children:`Send a message to the agent to see its activity here.`}),(0,d.jsxs)(`button`,{className:`btn btn-primary`,onClick:()=>t(`/app/agents/${encodeURIComponent(e)}/chat${p?`?user_id=${encodeURIComponent(p)}`:``}`),children:[(0,d.jsx)(`i`,{className:`fas fa-comment`}),` Chat with `,e]})]}):(0,d.jsx)(`div`,{children:h(w)})]})}export{g as default};