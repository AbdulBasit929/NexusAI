import{a as e}from"./chunk-Cyuzqnbw.js";import{n as t,t as n}from"./jsx-runtime-xIJTzhZo.js";import{t as r}from"./useModels-D5BUjD-W.js";var i=e(t(),1),a=n();function o({value:e,onChange:t,capability:n,placeholder:o=`Type or select a model...`,style:s,commitOnly:c=!1}){let{models:l,loading:u}=r(n),[d,f]=(0,i.useState)(``),[p,m]=(0,i.useState)(!1),[h,g]=(0,i.useState)(-1),_=(0,i.useRef)(null),v=(0,i.useRef)(null);(0,i.useEffect)(()=>{f(e||``)},[e]),(0,i.useEffect)(()=>{let e=e=>{_.current&&!_.current.contains(e.target)&&m(!1)};return document.addEventListener(`mousedown`,e),()=>document.removeEventListener(`mousedown`,e)},[]);let y=l.filter(e=>e.id.toLowerCase().includes(d.toLowerCase())),b=h>=0?h:y.length>0?0:-1,x=(0,i.useCallback)(e=>{f(c?``:e),t(e),m(!1),g(-1)},[t,c]);return(0,i.useEffect)(()=>{if(h>=0&&v.current){let e=v.current.children[h];e&&e.scrollIntoView({block:`nearest`})}},[h]),(0,a.jsxs)(`div`,{ref:_,className:`searchable-model-select`,style:s,children:[(0,a.jsx)(`style`,{children:`
        .searchable-model-select {
          position: relative;
          width: 280px;
        }
        .searchable-model-select input {
          width: 100%;
        }
        .sms-dropdown {
          position: absolute;
          top: 100%;
          left: 0;
          right: 0;
          z-index: 50;
          max-height: 220px;
          overflow-y: auto;
          background: var(--color-bg-primary);
          border: 1px solid var(--color-border);
          border-radius: var(--radius-md);
          box-shadow: var(--shadow-md);
          animation: dropdownIn 120ms ease-out;
          margin-top: 2px;
        }
        .sms-item {
          padding: 6px 10px;
          font-size: 0.8125rem;
          cursor: pointer;
          display: flex;
          align-items: center;
          gap: 6px;
        }
        .sms-item:hover, .sms-item.sms-focused {
          background: var(--color-bg-tertiary);
        }
        .sms-item.sms-active {
          color: var(--color-primary);
          font-weight: 600;
        }
        .sms-empty {
          padding: 8px 10px;
          font-size: 0.8125rem;
          color: var(--color-text-muted);
        }
      `}),(0,a.jsx)(`input`,{className:`input`,"aria-haspopup":`listbox`,"aria-expanded":p,value:d,onChange:e=>{f(e.target.value),m(!0),g(-1),c||t(e.target.value)},onFocus:()=>m(!0),onKeyDown:e=>{if(!p&&(e.key===`ArrowDown`||e.key===`ArrowUp`)){m(!0);return}if(!p&&e.key===`Enter`){e.preventDefault(),x(d);return}p&&(e.key===`ArrowDown`?(e.preventDefault(),g(e=>Math.min(e+1,y.length-1))):e.key===`ArrowUp`?(e.preventDefault(),g(e=>Math.max(e-1,0))):e.key===`Enter`?(e.preventDefault(),x(b>=0?y[b].id:d)):e.key===`Escape`&&(m(!1),g(-1)))},placeholder:u?`Loading models...`:o}),p&&!u&&(0,a.jsx)(`div`,{className:`sms-dropdown`,ref:v,role:`listbox`,children:y.length===0?(0,a.jsx)(`div`,{className:`sms-empty`,children:d?`No matching models — value will be used as-is`:`No models available`}):y.map((t,n)=>{let r=n===b;return(0,a.jsxs)(`div`,{role:`option`,"aria-selected":t.id===e,className:`sms-item${n===h||r?` sms-focused`:``}${t.id===e?` sms-active`:``}`,onMouseEnter:()=>g(n),onMouseDown:e=>{e.preventDefault(),x(t.id)},children:[(0,a.jsx)(`span`,{style:{flex:1,overflow:`hidden`,textOverflow:`ellipsis`,whiteSpace:`nowrap`},children:t.id}),r&&(0,a.jsx)(`span`,{style:{color:`var(--color-text-muted)`,fontSize:`0.75rem`,flexShrink:0},children:`↵`})]},t.id)})})]})}export{o as t};