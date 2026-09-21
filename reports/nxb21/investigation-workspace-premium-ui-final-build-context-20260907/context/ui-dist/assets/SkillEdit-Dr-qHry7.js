import{a as e}from"./chunk-Cyuzqnbw.js";import{n as t,t as n}from"./jsx-runtime-xIJTzhZo.js";import{D as r}from"./api-CfZtXBN5.js";import{D as i,E as a,O as o,T as s,k as c}from"./index-BdSjMiXm.js";import{t as l}from"./PageHeader-1P6MA6lR.js";var u=e(t(),1),d=n(),f=[`scripts/`,`references/`,`assets/`];function p(e){return f.some(t=>e.startsWith(t))&&!e.includes(`..`)}function m({title:e,icon:t,items:n,readOnly:r,pathPrefix:i,onView:a,onDelete:o,onUpload:s}){let[c,l]=(0,u.useState)(!1);return(0,d.jsxs)(`div`,{style:{marginBottom:`var(--spacing-lg)`},children:[(0,d.jsxs)(`div`,{style:{display:`flex`,justifyContent:`space-between`,alignItems:`center`,marginBottom:`var(--spacing-sm)`},children:[(0,d.jsxs)(`h3`,{style:{margin:0,fontWeight:600,fontSize:`0.95rem`,cursor:`pointer`,display:`flex`,alignItems:`center`,gap:`var(--spacing-xs)`},onClick:()=>l(e=>!e),children:[(0,d.jsx)(`i`,{className:`fas fa-chevron-${c?`right`:`down`}`,style:{fontSize:`0.75rem`,color:`var(--color-text-muted)`}}),(0,d.jsx)(`i`,{className:`fas fa-${t}`,style:{color:`var(--color-primary)`}}),` `,e,(0,d.jsx)(`span`,{className:`badge`,style:{marginLeft:`var(--spacing-xs)`},children:n.length})]}),!r&&(0,d.jsxs)(`button`,{className:`btn btn-primary btn-sm`,onClick:()=>s(i),children:[(0,d.jsx)(`i`,{className:`fas fa-upload`}),` Upload`]})]}),!c&&(n.length===0?(0,d.jsxs)(`p`,{style:{color:`var(--color-text-muted)`,fontSize:`0.875rem`,padding:`var(--spacing-sm)`},children:[`No `,e.toLowerCase(),` yet.`]}):(0,d.jsx)(`div`,{children:n.map(e=>(0,d.jsxs)(`div`,{className:`skilledit-resource-item`,children:[(0,d.jsxs)(`div`,{style:{minWidth:0},children:[(0,d.jsx)(`span`,{style:{fontWeight:500},children:e.name}),(0,d.jsxs)(`span`,{style:{color:`var(--color-text-secondary)`,fontSize:`0.8rem`,marginLeft:`var(--spacing-sm)`},children:[e.mime_type,` · `,(e.size||0).toLocaleString(),` B`]})]}),(0,d.jsxs)(`div`,{style:{display:`flex`,gap:`var(--spacing-xs)`},children:[(0,d.jsxs)(`button`,{className:`btn btn-secondary btn-sm`,onClick:()=>a(e),title:`View/Edit`,children:[(0,d.jsx)(`i`,{className:`fas fa-edit`}),` View/Edit`]}),!r&&(0,d.jsx)(`button`,{className:`btn btn-danger btn-sm`,onClick:()=>o(e.path),title:`Delete`,children:(0,d.jsx)(`i`,{className:`fas fa-trash`})})]})]},e.path))}))]})}function h({skillName:e,addToast:t}){let[n,i]=(0,u.useState)({scripts:[],references:[],assets:[],readOnly:!1}),[a,o]=(0,u.useState)(!0),[s,c]=(0,u.useState)({open:!1,path:``,name:``,content:``,readable:!0,saving:!1}),[l,f]=(0,u.useState)({open:!1,pathPrefix:`assets/`,file:null,pathInput:``,uploading:!1}),[h,g]=(0,u.useState)(null),_=async()=>{o(!0);try{let t=await r.listResources(e);i({scripts:t.scripts||[],references:t.references||[],assets:t.assets||[],readOnly:t.readOnly===!0})}catch(e){t(e.message||`Failed to load resources`,`error`)}finally{o(!1)}};(0,u.useEffect)(()=>{_()},[e]);let v=async n=>{if(c({open:!0,path:n.path,name:n.name,content:``,readable:n.readable!==!1,saving:!1}),n.readable!==!1)try{let t=await r.getResource(e,n.path,{json:!0}),i=t.encoding===`base64`&&t.content?atob(t.content):t.content||``;c(e=>({...e,content:i}))}catch(e){t(e.message||`Failed to load file`,`error`)}},y=async()=>{c(e=>({...e,saving:!0}));try{await r.updateResource(e,s.path,s.content),t(`Resource updated`,`success`),c(e=>({...e,open:!1})),_()}catch(e){t(e.message||`Update failed`,`error`)}finally{c(e=>({...e,saving:!1}))}},b=e=>{f({open:!0,pathPrefix:e,file:null,pathInput:``,uploading:!1})};return(0,d.jsxs)(d.Fragment,{children:[(0,d.jsxs)(`h3`,{style:{fontWeight:600,marginBottom:`var(--spacing-sm)`},children:[(0,d.jsx)(`i`,{className:`fas fa-folder`,style:{color:`var(--color-primary)`,marginRight:`var(--spacing-xs)`}}),` Resources`]}),(0,d.jsx)(`p`,{style:{color:`var(--color-text-secondary)`,fontSize:`0.875rem`,marginBottom:`var(--spacing-md)`},children:`Scripts, references, and assets for this skill. Paths must start with scripts/, references/, or assets/.`}),a?(0,d.jsx)(`div`,{style:{display:`flex`,justifyContent:`center`,padding:`var(--spacing-md)`},children:(0,d.jsx)(`i`,{className:`fas fa-spinner fa-spin`,style:{fontSize:`1.5rem`,color:`var(--color-text-muted)`}})}):(0,d.jsxs)(d.Fragment,{children:[(0,d.jsx)(m,{title:`Scripts`,icon:`code`,pathPrefix:`scripts/`,items:n.scripts,readOnly:n.readOnly,onView:v,onDelete:g,onUpload:b}),(0,d.jsx)(m,{title:`References`,icon:`book`,pathPrefix:`references/`,items:n.references,readOnly:n.readOnly,onView:v,onDelete:g,onUpload:b}),(0,d.jsx)(m,{title:`Assets`,icon:`image`,pathPrefix:`assets/`,items:n.assets,readOnly:n.readOnly,onView:v,onDelete:g,onUpload:b})]}),s.open&&(0,d.jsx)(`div`,{className:`skilledit-modal-overlay`,onClick:()=>!s.saving&&c(e=>({...e,open:!1})),children:(0,d.jsxs)(`div`,{className:`card skilledit-modal-card`,style:{maxWidth:`700px`},onClick:e=>e.stopPropagation(),children:[(0,d.jsxs)(`h3`,{style:{fontWeight:600,marginBottom:`var(--spacing-md)`},children:[(0,d.jsx)(`i`,{className:`fas fa-edit`,style:{color:`var(--color-primary)`,marginRight:`var(--spacing-xs)`}}),` Edit `,s.name]}),s.readable?(0,d.jsxs)(d.Fragment,{children:[(0,d.jsx)(`textarea`,{className:`input`,value:s.content,onChange:e=>c(t=>({...t,content:e.target.value})),rows:14,style:{fontFamily:`var(--font-mono)`,fontSize:`0.875rem`,marginBottom:`var(--spacing-md)`,width:`100%`}}),(0,d.jsxs)(`div`,{style:{display:`flex`,gap:`var(--spacing-sm)`,justifyContent:`flex-end`},children:[(0,d.jsx)(`button`,{className:`btn btn-secondary`,onClick:()=>c(e=>({...e,open:!1})),children:`Cancel`}),(0,d.jsx)(`button`,{className:`btn btn-primary`,disabled:s.saving,onClick:y,children:s.saving?(0,d.jsxs)(d.Fragment,{children:[(0,d.jsx)(`i`,{className:`fas fa-spinner fa-spin`}),` Saving...`]}):(0,d.jsxs)(d.Fragment,{children:[(0,d.jsx)(`i`,{className:`fas fa-save`}),` Save`]})})]})]}):(0,d.jsx)(`p`,{style:{color:`var(--color-text-secondary)`},children:`Binary file. Download via API or export skill.`})]})}),l.open&&(0,d.jsx)(`div`,{className:`skilledit-modal-overlay`,onClick:()=>!l.uploading&&f(e=>({...e,open:!1})),children:(0,d.jsxs)(`div`,{className:`card skilledit-modal-card`,style:{maxWidth:`400px`},onClick:e=>e.stopPropagation(),children:[(0,d.jsxs)(`h3`,{style:{fontWeight:600,marginBottom:`var(--spacing-md)`},children:[(0,d.jsx)(`i`,{className:`fas fa-upload`,style:{color:`var(--color-primary)`,marginRight:`var(--spacing-xs)`}}),` Upload to `,l.pathPrefix]}),(0,d.jsxs)(`div`,{className:`form-group`,children:[(0,d.jsx)(`label`,{className:`form-label`,children:`File`}),(0,d.jsx)(`input`,{type:`file`,className:`input`,onChange:e=>f(t=>({...t,file:e.target.files?.[0]||null}))})]}),(0,d.jsxs)(`div`,{className:`form-group`,children:[(0,d.jsxs)(`label`,{className:`form-label`,children:[`Path (default: `,l.pathPrefix,` + filename)`]}),(0,d.jsx)(`input`,{type:`text`,className:`input`,placeholder:`${l.pathPrefix}filename`,value:l.pathInput,onChange:e=>f(t=>({...t,pathInput:e.target.value}))})]}),(0,d.jsxs)(`div`,{style:{display:`flex`,gap:`var(--spacing-sm)`,justifyContent:`flex-end`,marginTop:`var(--spacing-md)`},children:[(0,d.jsx)(`button`,{className:`btn btn-secondary`,onClick:()=>f(e=>({...e,open:!1})),children:`Cancel`}),(0,d.jsx)(`button`,{className:`btn btn-primary`,disabled:l.uploading||!l.file,onClick:async()=>{let n=l.pathInput.trim()||(l.file?l.pathPrefix+l.file.name:``);if(!n||!l.file){t(`Select a file and ensure path is set`,`error`);return}if(!p(n)){t(`Path must start with scripts/, references/, or assets/`,`error`);return}f(e=>({...e,uploading:!0}));try{await r.createResource(e,n,l.file),t(`Resource added`,`success`),f(e=>({...e,open:!1})),_()}catch(e){t(e.message||`Upload failed`,`error`)}finally{f(e=>({...e,uploading:!1}))}},children:l.uploading?(0,d.jsxs)(d.Fragment,{children:[(0,d.jsx)(`i`,{className:`fas fa-spinner fa-spin`}),` Uploading...`]}):(0,d.jsxs)(d.Fragment,{children:[(0,d.jsx)(`i`,{className:`fas fa-upload`}),` Upload`]})})]})]})}),h&&(0,d.jsx)(`div`,{className:`skilledit-modal-overlay`,onClick:()=>g(null),children:(0,d.jsxs)(`div`,{className:`card skilledit-modal-card`,style:{maxWidth:`360px`},onClick:e=>e.stopPropagation(),children:[(0,d.jsxs)(`p`,{style:{marginBottom:`var(--spacing-md)`},children:[`Delete resource `,(0,d.jsx)(`strong`,{children:h}),`?`]}),(0,d.jsxs)(`div`,{style:{display:`flex`,gap:`var(--spacing-sm)`,justifyContent:`flex-end`},children:[(0,d.jsx)(`button`,{className:`btn btn-secondary`,onClick:()=>g(null),children:`Cancel`}),(0,d.jsxs)(`button`,{className:`btn btn-danger`,onClick:async()=>{if(h)try{await r.deleteResource(e,h),t(`Resource deleted`,`success`),g(null),_()}catch(e){t(e.message||`Delete failed`,`error`)}},children:[(0,d.jsx)(`i`,{className:`fas fa-trash`}),` Delete`]})]})]})})]})}function g(){let{name:e}=o(),t=s().pathname.endsWith(`/new`),n=e?decodeURIComponent(e):void 0,f=a(),{addToast:p}=i(),[m]=c(),g=m.get(`user_id`)||void 0,[_,v]=(0,u.useState)(!t),[y,b]=(0,u.useState)(!1),[x,S]=(0,u.useState)(`basic`),[C,w]=(0,u.useState)({name:``,description:``,content:``,license:``,compatibility:``,metadata:{},allowedTools:``});(0,u.useEffect)(()=>{if(t){v(!1);return}n&&r.get(n,g).then(e=>{w({name:e.name||``,description:e.description||``,content:e.content||``,license:e.license||``,compatibility:e.compatibility||``,metadata:e.metadata||{},allowedTools:e[`allowed-tools`]||``})}).catch(e=>{p(e.message||`Failed to load skill`,`error`),f(`/app/skills`)}).finally(()=>v(!1))},[t,n,f,p]);let T=async e=>{if(e.preventDefault(),!C.name.trim()){p(`Skill name is required`,`warning`);return}if(!C.description.trim()){p(`Skill description is required`,`warning`);return}b(!0);try{let e={name:C.name,description:C.description,content:C.content,license:C.license||void 0,compatibility:C.compatibility||void 0,metadata:Object.keys(C.metadata).length?C.metadata:void 0,"allowed-tools":C.allowedTools||void 0};t?(await r.create(e),p(`Skill created`,`success`)):(await r.update(n,{...e,name:void 0},g),p(`Skill updated`,`success`)),f(`/app/skills`)}catch(e){p(e.message||`Save failed`,`error`)}finally{b(!1)}};if(_)return(0,d.jsx)(`div`,{className:`page page--narrow`,style:{display:`flex`,justifyContent:`center`,padding:`var(--spacing-xl)`},children:(0,d.jsx)(`i`,{className:`fas fa-spinner fa-spin`,style:{fontSize:`2rem`,color:`var(--color-primary)`}})});let E=[{id:`basic`,label:`Basic information`,icon:`fa-info-circle`},{id:`content`,label:`Content`,icon:`fa-file-alt`},...!t&&n?[{id:`resources`,label:`Resources`,icon:`fa-folder`}]:[]];return(0,d.jsxs)(`div`,{className:`page page--narrow`,children:[(0,d.jsx)(`style`,{children:`
        .skilledit-back-link {
          display: inline-flex;
          align-items: center;
          gap: var(--spacing-xs);
          color: var(--color-text-secondary);
          font-size: 0.875rem;
          margin-bottom: var(--spacing-sm);
          cursor: pointer;
          text-decoration: none;
        }
        .skilledit-back-link:hover {
          color: var(--color-primary);
        }
        .skilledit-layout {
          display: flex;
          gap: var(--spacing-lg);
        }
        .skilledit-sidebar {
          flex-shrink: 0;
          width: 200px;
        }
        .skilledit-sidebar-nav {
          list-style: none;
          padding: 0;
          margin: 0;
        }
        .skilledit-sidebar-item {
          display: flex;
          align-items: center;
          gap: var(--spacing-sm);
          padding: var(--spacing-sm) var(--spacing-md);
          font-size: 0.875rem;
          color: var(--color-text-secondary);
          cursor: pointer;
          border-radius: var(--radius-md);
          border-left: 3px solid transparent;
          transition: all var(--duration-fast) var(--ease-default);
        }
        .skilledit-sidebar-item:hover {
          color: var(--color-text-primary);
          background: var(--color-primary-light);
        }
        .skilledit-sidebar-item.active {
          color: var(--color-primary);
          background: var(--color-primary-light);
          border-left-color: var(--color-primary);
          font-weight: 500;
        }
        .skilledit-form-area {
          flex: 1;
          min-width: 0;
        }
        .skilledit-section-title {
          font-weight: 600;
          font-size: 1rem;
          margin-bottom: var(--spacing-md);
        }
        .skilledit-field {
          margin-bottom: var(--spacing-md);
        }
        .skilledit-field label {
          display: block;
          font-size: 0.875rem;
          font-weight: 500;
          margin-bottom: var(--spacing-xs);
          color: var(--color-text-primary);
        }
        .skilledit-field .required {
          color: var(--color-error);
        }
        .skilledit-field .help-text {
          font-size: 0.8rem;
          color: var(--color-text-muted);
          margin-top: var(--spacing-xs);
        }
        .skilledit-form-actions {
          display: flex;
          gap: var(--spacing-sm);
          justify-content: flex-end;
          margin-top: var(--spacing-lg);
          padding-top: var(--spacing-md);
          border-top: 1px solid var(--color-border-subtle);
        }
        .skilledit-modal-overlay {
          position: fixed;
          inset: 0;
          background: var(--color-bg-overlay);
          z-index: 50;
          display: flex;
          align-items: center;
          justify-content: center;
          padding: var(--spacing-md);
        }
        .skilledit-modal-card {
          width: 100%;
          max-height: 90vh;
          display: flex;
          flex-direction: column;
          overflow: auto;
        }
        .skilledit-resource-item {
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
        @media (max-width: 700px) {
          .skilledit-layout {
            flex-direction: column;
          }
          .skilledit-sidebar {
            width: 100%;
          }
          .skilledit-sidebar-nav {
            display: flex;
            gap: var(--spacing-xs);
            overflow-x: auto;
          }
          .skilledit-sidebar-item {
            border-left: none;
            border-bottom: 3px solid transparent;
            white-space: nowrap;
          }
          .skilledit-sidebar-item.active {
            border-left-color: transparent;
            border-bottom-color: var(--color-primary);
          }
        }
      `}),(0,d.jsxs)(`a`,{className:`skilledit-back-link`,onClick:()=>f(`/app/skills`),children:[(0,d.jsx)(`i`,{className:`fas fa-arrow-left`}),` Back to skills`]}),(0,d.jsx)(l,{title:(0,d.jsxs)(d.Fragment,{children:[(0,d.jsx)(`i`,{className:`fas fa-book`,style:{marginRight:`var(--spacing-xs)`}}),` `,t?`New skill`:`Edit: ${n}`]})}),(0,d.jsx)(`div`,{className:`card`,style:{marginTop:`var(--spacing-md)`},children:(0,d.jsxs)(`div`,{className:`skilledit-layout`,children:[(0,d.jsx)(`div`,{className:`skilledit-sidebar`,children:(0,d.jsx)(`ul`,{className:`skilledit-sidebar-nav`,children:E.map(e=>(0,d.jsxs)(`li`,{className:`skilledit-sidebar-item ${x===e.id?`active`:``}`,onClick:()=>S(e.id),children:[(0,d.jsx)(`i`,{className:`fas ${e.icon}`}),` `,e.label]},e.id))})}),(0,d.jsx)(`div`,{className:`skilledit-form-area`,children:(0,d.jsxs)(`form`,{onSubmit:T,noValidate:!0,children:[(0,d.jsxs)(`div`,{style:{display:x===`basic`?`block`:`none`},children:[(0,d.jsx)(`h3`,{className:`skilledit-section-title`,children:`Basic information`}),(0,d.jsxs)(`div`,{className:`skilledit-field`,children:[(0,d.jsxs)(`label`,{htmlFor:`skill-name`,children:[`Name (lowercase, hyphens only) `,(0,d.jsx)(`span`,{className:`required`,children:`*`})]}),(0,d.jsx)(`input`,{id:`skill-name`,type:`text`,className:`input`,value:C.name,onChange:e=>w(t=>({...t,name:e.target.value})),required:!0,disabled:!t,placeholder:`my-skill`}),!t&&(0,d.jsx)(`p`,{className:`help-text`,children:`Name cannot be changed after creation.`})]}),(0,d.jsxs)(`div`,{className:`skilledit-field`,children:[(0,d.jsxs)(`label`,{htmlFor:`skill-desc`,children:[`Description (required, max 1024 chars) `,(0,d.jsx)(`span`,{className:`required`,children:`*`})]}),(0,d.jsx)(`textarea`,{id:`skill-desc`,className:`input`,value:C.description,onChange:e=>w(t=>({...t,description:e.target.value})),required:!0,maxLength:1024,rows:2})]}),(0,d.jsxs)(`div`,{className:`skilledit-field`,children:[(0,d.jsx)(`label`,{htmlFor:`skill-license`,children:`License (optional)`}),(0,d.jsx)(`input`,{id:`skill-license`,type:`text`,className:`input`,value:C.license,onChange:e=>w(t=>({...t,license:e.target.value}))})]}),(0,d.jsxs)(`div`,{className:`skilledit-field`,children:[(0,d.jsx)(`label`,{htmlFor:`skill-compat`,children:`Compatibility (optional, max 500 chars)`}),(0,d.jsx)(`input`,{id:`skill-compat`,type:`text`,className:`input`,value:C.compatibility,onChange:e=>w(t=>({...t,compatibility:e.target.value})),maxLength:500})]}),(0,d.jsxs)(`div`,{className:`skilledit-field`,children:[(0,d.jsx)(`label`,{htmlFor:`skill-allowed-tools`,children:`Allowed tools (optional)`}),(0,d.jsx)(`input`,{id:`skill-allowed-tools`,type:`text`,className:`input`,value:C.allowedTools,onChange:e=>w(t=>({...t,allowedTools:e.target.value})),placeholder:`tool1, tool2`})]})]}),(0,d.jsxs)(`div`,{style:{display:x===`content`?`block`:`none`},children:[(0,d.jsx)(`h3`,{className:`skilledit-section-title`,children:`Content`}),(0,d.jsxs)(`div`,{className:`skilledit-field`,children:[(0,d.jsx)(`label`,{htmlFor:`skill-content`,children:`Skill content (markdown)`}),(0,d.jsx)(`textarea`,{id:`skill-content`,className:`input`,value:C.content,onChange:e=>w(t=>({...t,content:e.target.value})),rows:14,style:{fontFamily:`var(--font-mono)`,fontSize:`0.875rem`}})]})]}),x===`resources`&&(0,d.jsx)(`div`,{children:t||!n?(0,d.jsxs)(`div`,{children:[(0,d.jsx)(`h3`,{className:`skilledit-section-title`,children:`Resources`}),(0,d.jsx)(`p`,{style:{color:`var(--color-text-secondary)`},children:`Save the skill first to add scripts, references, and assets. After creating the skill, use this tab to upload files and manage resources.`})]}):(0,d.jsx)(h,{skillName:n,addToast:p})}),(0,d.jsxs)(`div`,{className:`skilledit-form-actions`,children:[(0,d.jsxs)(`button`,{type:`button`,className:`btn btn-secondary`,onClick:()=>f(`/app/skills`),children:[(0,d.jsx)(`i`,{className:`fas fa-times`}),` Cancel`]}),(0,d.jsxs)(`button`,{type:`submit`,className:`btn btn-primary`,disabled:y,children:[(0,d.jsx)(`i`,{className:`fas fa-save`}),` `,y?`Saving...`:t?`Create skill`:`Save changes`]})]})]})})]})})]})}export{g as default};