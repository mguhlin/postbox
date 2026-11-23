'use strict';
document.addEventListener('DOMContentLoaded',()=>{
 document.getElementById('printNow').addEventListener('click',async()=>{
  const button=document.getElementById('printNow');button.disabled=true;
  try{
   await Promise.all([...document.images].map(image=>image.decode().catch(()=>{})));
   const response=await fetch('/desktop/print',{method:'POST',headers:{'X-Viewer-Token':document.querySelector('meta[name=viewer-token]').content,'Content-Type':'application/json'},body:JSON.stringify({id:document.querySelector('meta[name=print-id]').content})});
   if(!response.ok)throw Error('The system print dialog could not open.');
  }catch(e){const p=document.createElement('p');p.setAttribute('role','alert');p.textContent=e.message;button.after(p)}finally{button.disabled=false}
 });
});
