function t(i,n=2){if(i==null||i==="")return"-";const r=Number(i);return Number.isNaN(r)?String(i):r.toLocaleString("zh-CN",{minimumFractionDigits:n,maximumFractionDigits:n})}export{t as f};
