(async()=>{
 const wait=async(test)=>{for(let i=0;i<200;i++){if(test())return;await new Promise(r=>setTimeout(r,100))}throw Error('Native page did not become ready')};
 const checks=[];const check=(value,name)=>{if(!value)throw Error(name);checks.push(name)};
 try{
  await wait(()=>typeof window.POSTBOX_DESKTOP!=='undefined'&&typeof openFiles==='function'&&document.getElementById('count'));
  await window.POSTBOX_DESKTOP.drop(window.POSTBOX_SMOKE_DROP_TICKET);check(messages.length===1,'Native single-use drop capability imports a copy');await $('clear').onclick();
  await openFiles();check(messages.length===1,'Native Open streams an EML into the shared mail engine');
  const m=messages[0];check(matchesQuery(m,'body:budget has:attachment'),'Search includes HTML text and attachment names');check(m.hasHtml,'HTML import');
  check(getComputedStyle(document.querySelector('header')).backgroundColor!=='rgb(25, 63, 57)','Theme stylesheet loaded');
  show(m);selectedMessages.add(m.id);render();
  await window.POSTBOX_DESKTOP.save(m.id,'0');checks.push('Native attachment save');
  await exportEmails('eml');await exportEmails('mbox');checks.push('EML ZIP and MBOX exports');
  await exportAttachments(true);checks.push('Organized folder export');
  await previewAttachment(m,m.attachments[0],0);await wait(()=>document.querySelector('#previewContent pre'));check(document.querySelector('#previewContent').textContent.includes('Attachment content'),'Text attachment preview');$('previewDialog').close();
  await previewAttachment(m,m.attachments[1],1);await wait(()=>document.querySelector('#previewContent iframe')?.dataset.pdfReady);const pdfFrame=document.querySelector('#previewContent iframe');check(pdfFrame.dataset.pdfReady==='true','Bundled PDF attachment renderer: '+(pdfFrame.dataset.pdfError||''));$('previewDialog').close();
  openPdfOptions(true);await previewPdf();checks.push('Native print preview creation');
  const denied=await fetch('/desktop/save',{method:'POST',headers:{'Content-Type':'application/json'},body:'{}'});check(denied.status===403,'Native bridge rejects missing token');
  const result=await window.POSTBOX_DESKTOP.delete({action:'attachments',attachments:{[m.id]:[0,1]},messageIds:[],originals:true});check(result.results.length===1&&!result.error,'Atomic native original-file attachment removal');
  await $('clear').onclick();check(!messages.length&&!$('body').textContent&&!$('meta').textContent&&!$('subject').textContent,'Clear removes server records and reader DOM');
  await openFiles();check(messages.length===1&&messages[0].attachments.length===0,'Original edit survives clear and reimport');
  const removed=await window.POSTBOX_DESKTOP.delete({action:'messages',messageIds:[messages[0].id],attachments:{},originals:true});check(removed.results.length===1&&!removed.error,'Native original-file deletion');
  await window.POSTBOX_DESKTOP.call('smoke-result',{result:{ok:true,checks,userAgent:navigator.userAgent}});
 }catch(e){await window.POSTBOX_DESKTOP.call('smoke-result',{result:{ok:false,error:e.message,stack:e.stack,checks}})}
})();
