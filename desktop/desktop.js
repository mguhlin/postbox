'use strict';
window.POSTBOX_DESKTOP={
 async call(action,payload={}){
  const response=await fetch('/desktop/'+action,{method:'POST',headers:{'X-Viewer-Token':window.VIEWER_TOKEN,'Content-Type':'application/json'},body:JSON.stringify(payload)});
  let result;try{result=await response.json()}catch(e){throw Error('The desktop action failed. Close and reopen Postbox if the mail engine stopped.')}
  if(!response.ok)throw Error(result.error||'Desktop action failed.');return result;
 },
 async open(folder){return this.import('open',{folder})},
 async drop(ticket){return this.import('drop',{ticket})},
 async import(action,payload){
  document.body.classList.remove('drag');
  if(busy){setStatus('Finish the current operation, then open or drop the files again.');return}setBusy(true);setStatus('Opening email exports…');
  try{const result=await this.call(action,payload);let count=0;
   for(const imported of result.imports){messages.push(...imported);count+=imported.length}
   if(!selected&&messages.length)show(messages[0]);else render();
   $('importReport').textContent=result.errors.join('\n');$('importReport').hidden=!result.errors.length;
   setStatus(`${count} messages opened${result.errors.length?`; ${result.errors.length} files failed`:''}. Files stay on this computer.`);
  }catch(e){setStatus(e.message)}finally{setBusy(false)}
 },
 async exportEmails(format){
  if(busy||!selectedMessages.size)return;setBusy(true);
  try{const result=await this.call('export',{route:'/export-emails',payload:{messageIds:[...selectedMessages],format}});setStatus(result.cancelled?'Export cancelled.':`Saved ${selectedMessages.size} email(s) as ${format==='mbox'?'MBOX':'EML ZIP'}.`)}catch(e){setStatus(e.message)}finally{setBusy(false)}
 },
 async exportAttachments(all){
  if(busy)return;const attachments=attachmentSelection(all);if(!Object.keys(attachments).length){setStatus('No attachments in this selection.');return}
  setBusy(true);try{const result=await this.call('export-folder',{attachments});setStatus(result.cancelled?'Export cancelled.':`Exported ${result.saved} attachments by year/month/email. Existing files were kept.`)}catch(e){setStatus(e.message)}finally{setBusy(false)}
 },
 async save(id,part){if(busy)return;setBusy(true);try{const result=await this.call('save',{id,part});setStatus(result.cancelled?'Save cancelled.':'File saved.')}catch(e){setStatus(e.message)}finally{setBusy(false)}},
 async print(){if(busy)return;$('pdfDialog').close();setBusy(true);try{await this.call('preview',{payload:{messageIds:pdfMessageIds,clean:$('pdfClean').checked,paper:$('pdfPaper').value}});setStatus('Print preview ready in a Postbox window. Choose Print / Save as PDF to open the system print dialog.')}catch(e){setStatus(e.message)}finally{setBusy(false)}},
 delete(payload){return this.call('delete',payload)}
};
document.addEventListener('DOMContentLoaded',()=>{
 document.body.setAttribute('data-file-drop-target','');
 $('folder').textContent='Open managed folder';$('pdfPreview').textContent='Open desktop print preview';
 document.querySelector('.bulk-note').textContent='Filter by dates, select results, or Shift-click checkboxes. The desktop Open controls manage originals. Imported copies can reconnect to a chosen original folder before editing. Session-only deletion keeps originals. Exported copies remain on disk.';
 document.querySelector('.preview-note').textContent='Documents show text previews. PDF and media playback depend on the system webview and codecs. Save a file to open it in another application.';
 document.addEventListener('click',event=>{
  const link=event.target.closest('a');if(!link)return;
  const match=new RegExp('^/'+window.VIEWER_TOKEN+'/([a-f0-9]+)/([0-9]+|original)$').exec(new URL(link.href).pathname);
  if(match){event.preventDefault();window.POSTBOX_DESKTOP.save(match[1],match[2])}
  // External links are explicit choices. Wails opens them in the system browser;
  // no app workflow needs them, and mail HTML has neither links nor bridge access.
 });
});
