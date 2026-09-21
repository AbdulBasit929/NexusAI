import{a as e}from"./chunk-Cyuzqnbw.js";import{n as t,t as n}from"./jsx-runtime-xIJTzhZo.js";import{n as r}from"./api-CfZtXBN5.js";import{y as i}from"./index-u1ILYHtj.js";var a=e(t(),1);function o(){let{isAdmin:e,authEnabled:t}=i(),[n,o]=(0,a.useState)({});return(0,a.useEffect)(()=>{if(!e||!t)return;let n=!1;return r.list().then(e=>{if(n)return;let t=Array.isArray(e)?e:e?.users||[],r={};for(let e of t)r[e.id]={name:e.name||e.email||e.id,email:e.email,avatarUrl:e.avatar_url};o(r)}).catch(()=>{}),()=>{n=!0}},[e,t]),n}var s=n();function c({title:e,userGroups:t,userMap:n,currentUserId:r,renderGroup:i,itemKey:o}){let[c,u]=(0,a.useState)(!1);if(!t||Object.keys(t).length===0)return null;let d=Object.keys(t).filter(e=>e!==r);if(d.length===0)return null;let f=d.length;return(0,s.jsxs)(`div`,{style:{marginTop:`var(--spacing-lg)`},children:[(0,s.jsx)(`style`,{children:`
        .ugs-header {
          display: flex;
          align-items: center;
          gap: var(--spacing-sm);
          cursor: pointer;
          padding: var(--spacing-sm) 0;
          border-top: 1px solid var(--color-border-subtle);
          user-select: none;
        }
        .ugs-header:hover { opacity: 0.8; }
        .ugs-chevron {
          transition: transform 0.2s;
          font-size: 0.75rem;
          color: var(--color-text-muted);
        }
        .ugs-chevron.open { transform: rotate(90deg); }
        .ugs-title {
          font-weight: 600;
          font-size: 0.875rem;
          color: var(--color-text-secondary);
        }
        .ugs-badge {
          font-size: 0.75rem;
          background: var(--color-bg-tertiary);
          color: var(--color-text-muted);
          padding: 2px 8px;
          border-radius: var(--radius-sm);
        }
        .ugs-content {
          background: var(--color-bg-secondary);
          border: 1px solid var(--color-border-subtle);
          border-radius: var(--radius-lg);
          padding: var(--spacing-md);
          margin-top: var(--spacing-sm);
        }
        .ugs-user-section {
          margin-bottom: var(--spacing-md);
        }
        .ugs-user-section:last-child { margin-bottom: 0; }
        .ugs-user-header {
          display: flex;
          align-items: center;
          gap: var(--spacing-sm);
          margin-bottom: var(--spacing-sm);
          cursor: pointer;
        }
        .ugs-avatar {
          width: 24px;
          height: 24px;
          border-radius: 50%;
          background: var(--color-primary);
          color: white;
          display: flex;
          align-items: center;
          justify-content: center;
          font-size: 0.6875rem;
          font-weight: 600;
          flex-shrink: 0;
        }
        .ugs-avatar img {
          width: 100%;
          height: 100%;
          border-radius: 50%;
          object-fit: cover;
        }
        .ugs-user-name {
          font-weight: 500;
          font-size: 0.8125rem;
        }
        .ugs-user-count {
          font-size: 0.75rem;
          color: var(--color-text-muted);
        }
      `}),(0,s.jsxs)(`div`,{className:`ugs-header`,role:`button`,tabIndex:0,onClick:()=>u(e=>!e),onKeyDown:e=>{(e.key===`Enter`||e.key===` `)&&(e.preventDefault(),u(e=>!e))},"aria-expanded":c,children:[(0,s.jsx)(`i`,{className:`fas fa-chevron-right ugs-chevron ${c?`open`:``}`}),(0,s.jsx)(`span`,{className:`ugs-title`,children:e}),(0,s.jsxs)(`span`,{className:`ugs-badge`,children:[f,` user`,f===1?``:`s`]})]}),c&&(0,s.jsx)(`div`,{className:`ugs-content`,children:d.map(e=>{let r=n[e]||{},a=r.name||r.email||e.slice(0,8)+`...`,c=(a[0]||`?`).toUpperCase(),u=t[e],d=o?u[o]:u,f=Array.isArray(d)?d.length:0;return(0,s.jsx)(l,{uid:e,displayName:a,initials:c,avatarUrl:r.avatarUrl,count:f,itemKey:o,children:i(d,e)},e)})})]})}function l({uid:e,displayName:t,initials:n,avatarUrl:r,count:i,itemKey:o,children:c}){let[l,u]=(0,a.useState)(!0);return(0,s.jsxs)(`div`,{className:`ugs-user-section`,children:[(0,s.jsxs)(`div`,{className:`ugs-user-header`,role:`button`,tabIndex:0,onClick:()=>u(e=>!e),onKeyDown:e=>{(e.key===`Enter`||e.key===` `)&&(e.preventDefault(),u(e=>!e))},"aria-expanded":l,children:[(0,s.jsx)(`i`,{className:`fas fa-chevron-right ugs-chevron ${l?`open`:``}`,style:{fontSize:`0.625rem`}}),(0,s.jsx)(`div`,{className:`ugs-avatar`,children:r?(0,s.jsx)(`img`,{src:r,alt:``}):n}),(0,s.jsx)(`span`,{className:`ugs-user-name`,children:t}),(0,s.jsxs)(`span`,{className:`ugs-user-count`,children:[i,` `,o||`item`,i===1?``:`s`]})]}),l&&c]})}export{o as n,c as t};