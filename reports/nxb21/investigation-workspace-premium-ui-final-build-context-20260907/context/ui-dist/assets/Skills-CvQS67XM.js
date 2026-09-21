import{a as e}from"./chunk-Cyuzqnbw.js";import{n as t,t as n}from"./jsx-runtime-xIJTzhZo.js";import{D as r}from"./api-CfZtXBN5.js";import{D as i,E as a,c as o,y as s}from"./index-BdSjMiXm.js";import{t as c}from"./ConfirmDialog-BMRAwYAU.js";import{t as l}from"./PageHeader-1P6MA6lR.js";import{n as u,t as d}from"./UserGroupSection-CzfHXHWr.js";var f=e(t(),1),p=n();function m(){let{addToast:e}=i(),t=a(),{t:n}=o(`skills`),{isAdmin:m,authEnabled:h,user:g}=s(),_=u(),[v,y]=(0,f.useState)([]),[b,x]=(0,f.useState)(``),[S,C]=(0,f.useState)(!0),[w,T]=(0,f.useState)(!1),[E,D]=(0,f.useState)(!1),[O,k]=(0,f.useState)(!1),[A,j]=(0,f.useState)([]),[M,N]=(0,f.useState)(``),[P,F]=(0,f.useState)(!1),[I,L]=(0,f.useState)(null),[R,z]=(0,f.useState)(null),[B,V]=(0,f.useState)(null),H=(0,f.useCallback)(async()=>{C(!0),D(!1);let t=e=>Promise.race([e,new Promise((e,t)=>setTimeout(()=>t(Error(`Request timed out`)),15e3))]);try{if(b.trim()){let e=await t(r.search(b.trim()));y(Array.isArray(e)?e:[]),z(null)}else{let e=await t(r.list(m&&h));Array.isArray(e)?(y(e),z(null)):(y(Array.isArray(e.skills)?e.skills:[]),z(e.user_groups||null))}}catch(t){t.message?.includes(`503`)||t.message?.includes(`skills`)?(D(!0),y([])):(e(t.message||n(`toasts.loadFailed`),`error`),y([]))}finally{C(!1)}},[b,e,m,h,n]);(0,f.useEffect)(()=>{H()},[H]);let U=async(t,i)=>{V({title:n(`deleteDialog.title`),message:n(`deleteDialog.message`,{name:t}),confirmLabel:n(`deleteDialog.confirm`),danger:!0,onConfirm:async()=>{V(null);try{await r.delete(t,i),e(n(`toasts.deleted`,{name:t}),`success`),H()}catch(t){e(t.message||n(`toasts.deleteFailed`),`error`)}}})},W=async(t,i)=>{try{let a=r.exportUrl(t,i),o=await fetch(a,{credentials:`same-origin`});if(!o.ok)throw Error(o.statusText||`Export failed`);let s=await o.blob(),c=document.createElement(`a`);c.href=URL.createObjectURL(s),c.download=`${t.replace(/\//g,`-`)}.tar.gz`,document.body.appendChild(c),c.click(),document.body.removeChild(c),URL.revokeObjectURL(c.href),e(n(`toasts.exported`,{name:t}),`success`)}catch(t){e(t.message||n(`toasts.exportFailed`),`error`)}},G=async t=>{let i=t.target.files?.[0];if(i){T(!0);try{await r.import(i),e(n(`toasts.imported`,{file:i.name}),`success`),H()}catch(t){e(t.message||n(`toasts.importFailed`),`error`)}finally{T(!1),t.target.value=``}}},K=async()=>{F(!0);try{let e=await r.listGitRepos();j(Array.isArray(e)?e:[])}catch(t){e(t.message||n(`toasts.loadReposFailed`),`error`),j([])}finally{F(!1)}};(0,f.useEffect)(()=>{O&&K()},[O]);let q=async t=>{t.preventDefault();let i=M.trim();if(i){L(`add`);try{await r.addGitRepo(i),N(``),await K(),H(),e(n(`toasts.repoAdded`),`success`)}catch(t){e(t.message||n(`toasts.addRepoFailed`),`error`)}finally{L(null)}}},J=async t=>{L(t);try{await r.syncGitRepo(t),await K(),H(),e(n(`toasts.synced`),`success`)}catch(t){e(t.message||n(`toasts.syncFailed`),`error`)}finally{L(null)}},Y=async t=>{try{await r.toggleGitRepo(t),await K(),H(),e(n(`toasts.toggled`),`success`)}catch(t){e(t.message||n(`toasts.toggleFailed`),`error`)}},X=async t=>{V({title:n(`removeRepoDialog.title`),message:n(`removeRepoDialog.message`),confirmLabel:n(`removeRepoDialog.confirm`),danger:!0,onConfirm:async()=>{V(null);try{await r.deleteGitRepo(t),await K(),H(),e(n(`toasts.removed`),`success`)}catch(t){e(t.message||n(`toasts.removeFailed`),`error`)}}})};return E?(0,p.jsxs)(`div`,{className:`page page--wide`,children:[(0,p.jsx)(l,{title:n(`title`),supporting:n(`unavailable.subtitle`)}),(0,p.jsx)(`div`,{style:{display:`flex`,justifyContent:`center`,padding:`var(--spacing-xl)`},children:(0,p.jsxs)(`button`,{className:`btn btn-primary`,onClick:()=>{D(!1),H()},children:[(0,p.jsx)(`i`,{className:`fas fa-redo`}),` `,n(`unavailable.retry`)]})})]}):(0,p.jsxs)(`div`,{className:`page page--wide`,children:[(0,p.jsx)(`style`,{children:`
        .skills-header-actions {
          display: flex;
          gap: var(--spacing-sm);
          align-items: center;
          flex-wrap: wrap;
        }
        .skills-import-input {
          display: none;
        }
        .skills-grid {
          display: grid;
          grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
          gap: var(--spacing-md);
        }
        .skills-card-header {
          display: flex;
          justify-content: space-between;
          align-items: flex-start;
          margin-bottom: var(--spacing-sm);
        }
        .skills-card-name {
          font-size: 1.05rem;
          font-weight: 600;
          margin: 0;
          word-break: break-word;
        }
        .skills-card-desc {
          margin: 0 0 var(--spacing-md) 0;
          color: var(--color-text-secondary);
          font-size: 0.875rem;
        }
        .skills-card-actions {
          display: flex;
          gap: var(--spacing-xs);
          flex-wrap: wrap;
        }
        .skills-git-section {
          margin-bottom: var(--spacing-lg);
          padding: var(--spacing-md);
          background: var(--color-bg-secondary);
          border: 1px solid var(--color-border-default);
          border-radius: var(--radius-lg);
        }
        .skills-git-title {
          font-size: 1rem;
          font-weight: 600;
          margin: 0 0 var(--spacing-sm) 0;
        }
        .skills-git-desc {
          color: var(--color-text-secondary);
          font-size: 0.875rem;
          margin-bottom: var(--spacing-md);
        }
        .skills-git-form {
          display: flex;
          gap: var(--spacing-sm);
          flex-wrap: wrap;
          margin-bottom: var(--spacing-md);
        }
        .skills-git-form .input {
          flex: 1;
          min-width: 200px;
        }
        .skills-git-repo-item {
          display: flex;
          align-items: center;
          justify-content: space-between;
          flex-wrap: wrap;
          gap: var(--spacing-sm);
          padding: var(--spacing-sm) var(--spacing-md);
          margin-bottom: var(--spacing-xs);
          background: var(--color-bg-tertiary);
          border: 1px solid var(--color-border-subtle);
          border-radius: var(--radius-md);
        }
        .skills-git-repo-name {
          font-weight: 600;
        }
        .skills-git-repo-url {
          color: var(--color-text-secondary);
          font-size: 0.875rem;
          margin-left: var(--spacing-sm);
        }
        .skills-git-repo-actions {
          display: flex;
          gap: var(--spacing-xs);
        }
      `}),(0,p.jsx)(l,{title:n(`title`),supporting:n(`subtitle`),actions:(0,p.jsxs)(`div`,{className:`skills-header-actions`,children:[(0,p.jsx)(`input`,{type:`text`,className:`input`,placeholder:n(`search.placeholder`),value:b,onChange:e=>x(e.target.value),style:{width:`200px`}}),(0,p.jsxs)(`button`,{className:`btn btn-primary`,onClick:()=>t(`/app/skills/new`),children:[(0,p.jsx)(`i`,{className:`fas fa-plus`}),` `,n(`actions.newSkill`)]}),(0,p.jsxs)(`label`,{className:`btn btn-secondary`,style:{cursor:`pointer`},children:[(0,p.jsx)(`i`,{className:`fas fa-file-import`}),` `,n(w?`actions.importing`:`actions.import`),(0,p.jsx)(`input`,{type:`file`,accept:`.tar.gz`,className:`skills-import-input`,onChange:G,disabled:w})]}),(0,p.jsxs)(`button`,{className:`btn ${O?`btn-primary`:`btn-secondary`}`,onClick:()=>k(e=>!e),children:[(0,p.jsx)(`i`,{className:`fas fa-code-branch`}),` `,n(`actions.gitRepos`)]})]})}),O&&(0,p.jsxs)(`div`,{className:`skills-git-section`,children:[(0,p.jsxs)(`h2`,{className:`skills-git-title`,children:[(0,p.jsx)(`i`,{className:`fas fa-code-branch`,style:{marginRight:`var(--spacing-xs)`,color:`var(--color-primary)`}}),` `,n(`git.title`)]}),(0,p.jsx)(`p`,{className:`skills-git-desc`,children:n(`git.description`)}),(0,p.jsxs)(`form`,{onSubmit:q,className:`skills-git-form`,children:[(0,p.jsx)(`input`,{type:`url`,className:`input`,placeholder:n(`git.urlPlaceholder`),value:M,onChange:e=>N(e.target.value)}),(0,p.jsx)(`button`,{type:`submit`,className:`btn btn-primary`,disabled:I===`add`,children:I===`add`?(0,p.jsxs)(p.Fragment,{children:[(0,p.jsx)(`i`,{className:`fas fa-spinner fa-spin`}),` `,n(`actions.adding`)]}):n(`actions.addRepo`)})]}),P?(0,p.jsx)(`div`,{style:{display:`flex`,justifyContent:`center`,padding:`var(--spacing-md)`},children:(0,p.jsx)(`i`,{className:`fas fa-spinner fa-spin`,style:{fontSize:`1.5rem`,color:`var(--color-text-muted)`}})}):A.length===0?(0,p.jsx)(`p`,{style:{color:`var(--color-text-secondary)`,fontSize:`0.875rem`},children:n(`git.noRepos`)}):(0,p.jsx)(`div`,{children:A.map(e=>(0,p.jsxs)(`div`,{className:`skills-git-repo-item`,children:[(0,p.jsxs)(`div`,{children:[(0,p.jsx)(`span`,{className:`skills-git-repo-name`,children:e.name||e.url}),(0,p.jsx)(`span`,{className:`skills-git-repo-url`,children:e.url}),!e.enabled&&(0,p.jsx)(`span`,{className:`badge`,style:{marginLeft:`var(--spacing-sm)`},children:n(`git.disabled`)})]}),(0,p.jsxs)(`div`,{className:`skills-git-repo-actions`,children:[(0,p.jsx)(`button`,{className:`btn btn-secondary btn-sm`,onClick:()=>J(e.id),disabled:I===e.id,title:n(`actions.sync`),children:I===e.id?(0,p.jsx)(`i`,{className:`fas fa-spinner fa-spin`}):(0,p.jsxs)(p.Fragment,{children:[(0,p.jsx)(`i`,{className:`fas fa-sync-alt`}),` `,n(`actions.sync`)]})}),(0,p.jsx)(`button`,{className:`btn btn-secondary btn-sm`,onClick:()=>Y(e.id),title:e.enabled?n(`actions.disable`):n(`actions.enable`),children:(0,p.jsx)(`i`,{className:`fas fa-toggle-${e.enabled?`on`:`off`}`})}),(0,p.jsx)(`button`,{className:`btn btn-danger btn-sm`,onClick:()=>X(e.id),title:n(`git.removeRepo`),children:(0,p.jsx)(`i`,{className:`fas fa-trash`})})]})]},e.id))})]}),S?(0,p.jsx)(`div`,{style:{display:`flex`,justifyContent:`center`,padding:`var(--spacing-xl)`},children:(0,p.jsx)(`i`,{className:`fas fa-spinner fa-spin`,style:{fontSize:`2rem`,color:`var(--color-primary)`}})}):v.length===0&&!R?(0,p.jsxs)(`div`,{className:`empty-state`,children:[(0,p.jsx)(`div`,{className:`empty-state-icon`,children:(0,p.jsx)(`i`,{className:`fas fa-book`})}),(0,p.jsx)(`h2`,{className:`empty-state-title`,children:n(`empty.title`)}),(0,p.jsx)(`p`,{className:`empty-state-text`,children:n(`empty.text`)}),(0,p.jsxs)(`div`,{style:{display:`flex`,gap:`var(--spacing-sm)`,justifyContent:`center`},children:[(0,p.jsxs)(`button`,{className:`btn btn-primary`,onClick:()=>t(`/app/skills/new`),children:[(0,p.jsx)(`i`,{className:`fas fa-plus`}),` `,n(`actions.createSkill`)]}),(0,p.jsxs)(`label`,{className:`btn btn-secondary`,style:{cursor:`pointer`},children:[(0,p.jsx)(`i`,{className:`fas fa-file-import`}),` `,n(`actions.import`),(0,p.jsx)(`input`,{type:`file`,accept:`.tar.gz`,className:`skills-import-input`,onChange:G,disabled:w})]})]})]}):(0,p.jsxs)(p.Fragment,{children:[R&&(0,p.jsx)(`h2`,{style:{fontSize:`1.1rem`,fontWeight:600,marginBottom:`var(--spacing-md)`},children:n(`sections.yourSkills`)}),v.length===0?(0,p.jsx)(`p`,{style:{color:`var(--color-text-secondary)`,marginBottom:`var(--spacing-md)`},children:n(`empty.noPersonal`)}):(0,p.jsx)(`div`,{className:`skills-grid`,children:v.map(e=>(0,p.jsxs)(`div`,{className:`card`,children:[(0,p.jsxs)(`div`,{className:`skills-card-header`,children:[(0,p.jsx)(`h3`,{className:`skills-card-name`,children:e.name}),e.readOnly&&(0,p.jsx)(`span`,{className:`badge`,children:n(`card.readOnly`)})]}),(0,p.jsx)(`p`,{className:`skills-card-desc`,children:e.description||n(`card.noDescription`)}),(0,p.jsxs)(`div`,{className:`skills-card-actions`,children:[!e.readOnly&&(0,p.jsxs)(`button`,{className:`btn btn-secondary btn-sm`,onClick:()=>t(`/app/skills/edit/${encodeURIComponent(e.name)}`),title:n(`card.editTitle`),children:[(0,p.jsx)(`i`,{className:`fas fa-edit`}),` `,n(`actions.edit`)]}),!e.readOnly&&(0,p.jsxs)(`button`,{className:`btn btn-danger btn-sm`,onClick:()=>U(e.name),title:n(`card.deleteTitle`),children:[(0,p.jsx)(`i`,{className:`fas fa-trash`}),` `,n(`actions.delete`)]}),(0,p.jsxs)(`button`,{className:`btn btn-secondary btn-sm`,onClick:()=>W(e.name),title:n(`card.exportTitle`),children:[(0,p.jsx)(`i`,{className:`fas fa-download`}),` `,n(`actions.export`)]})]})]},e.name))})]}),(0,p.jsx)(c,{open:!!B,title:B?.title,message:B?.message,confirmLabel:B?.confirmLabel,danger:B?.danger,onConfirm:B?.onConfirm,onCancel:()=>V(null)}),R&&(0,p.jsx)(d,{title:n(`sections.otherUsersSkills`),userGroups:R,userMap:_,currentUserId:g?.id,itemKey:`skills`,renderGroup:(e,r)=>(0,p.jsx)(`div`,{className:`skills-grid`,children:(e||[]).map(e=>(0,p.jsxs)(`div`,{className:`card`,children:[(0,p.jsxs)(`div`,{className:`skills-card-header`,children:[(0,p.jsx)(`h3`,{className:`skills-card-name`,children:e.name}),e.readOnly&&(0,p.jsx)(`span`,{className:`badge`,children:n(`card.readOnly`)})]}),(0,p.jsx)(`p`,{className:`skills-card-desc`,children:e.description||n(`card.noDescription`)}),(0,p.jsxs)(`div`,{className:`skills-card-actions`,children:[!e.readOnly&&(0,p.jsxs)(`button`,{className:`btn btn-secondary btn-sm`,onClick:()=>t(`/app/skills/edit/${encodeURIComponent(e.name)}?user_id=${encodeURIComponent(r)}`),title:n(`card.editTitle`),children:[(0,p.jsx)(`i`,{className:`fas fa-edit`}),` `,n(`actions.edit`)]}),!e.readOnly&&(0,p.jsxs)(`button`,{className:`btn btn-danger btn-sm`,onClick:()=>U(e.name,r),title:n(`card.deleteTitle`),children:[(0,p.jsx)(`i`,{className:`fas fa-trash`}),` `,n(`actions.delete`)]}),(0,p.jsxs)(`button`,{className:`btn btn-secondary btn-sm`,onClick:()=>W(e.name,r),title:n(`card.exportTitle`),children:[(0,p.jsx)(`i`,{className:`fas fa-download`}),` `,n(`actions.export`)]})]})]},e.name))})})]})}export{m as default};