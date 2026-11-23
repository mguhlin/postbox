'use strict';
window.addEventListener('error',event=>{
 window.previewError=event.message||'A local PDF module could not load.';
 parent.postMessage({type:'postbox-pdf-preview',ready:false,error:window.previewError},'*');
 const status=document.getElementById('status');if(status){status.dataset.failed='true';status.textContent='PDF preview unavailable. Save the original attachment to open it in a compatible PDF reader.'}
});
