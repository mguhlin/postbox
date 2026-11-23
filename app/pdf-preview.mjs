import * as pdfjs from '/vendor/pdfjs/build/pdf.mjs';
pdfjs.GlobalWorkerOptions.workerSrc='/vendor/pdfjs/build/pdf.worker.mjs';
const status=document.getElementById('status'),pages=document.getElementById('pages'),more=document.getElementById('more');
try{
 const source=new URLSearchParams(location.search).get('source')||'';
 if(!/^\/[A-Za-z0-9_-]+\/[a-f0-9]+\/preview\/\d+$/.test(source))throw Error('Invalid PDF attachment.');
 const task=pdfjs.getDocument({url:source,isEvalSupported:false,useWasm:false,disableAutoFetch:true,disableStream:true,cMapUrl:'/vendor/pdfjs/cmaps/',cMapPacked:true,standardFontDataUrl:'/vendor/pdfjs/standard_fonts/'});
 const pdf=await task.promise;let next=1;
 async function batch(){more.hidden=true;const last=Math.min(next+9,pdf.numPages);
  for(;next<=last;next++){
   status.textContent=`Rendering page ${next} of ${pdf.numPages}…`;
   const page=await pdf.getPage(next),base=page.getViewport({scale:1}),scale=Math.min(1.5,1800/base.width,2400/base.height),viewport=page.getViewport({scale});
   const section=document.createElement('section'),heading=document.createElement('h2'),canvas=document.createElement('canvas');heading.textContent=`Page ${next}`;section.append(heading,canvas);pages.append(section);
   canvas.width=viewport.width;canvas.height=viewport.height;canvas.setAttribute('role','img');canvas.setAttribute('aria-label',`PDF page ${next}; extracted text follows`);
   await page.render({canvasContext:canvas.getContext('2d'),viewport}).promise;
   const details=document.createElement('details'),summary=document.createElement('summary'),text=document.createElement('p');summary.textContent='Read extracted page text';text.textContent=(await page.getTextContent()).items.map(item=>item.str||'').join(' ')||'No extractable text. This page may be a scanned image.';details.append(summary,text);section.append(details);page.cleanup();
  }
  more.hidden=next>pdf.numPages;status.textContent=`${next-1} of ${pdf.numPages} PDF pages shown. Content is processed locally; PDF scripts and embedded actions are not run.`;
 }
 more.onclick=()=>batch().catch(showError);await batch();parent.postMessage({type:"postbox-pdf-preview",ready:true},"*");
}catch(error){showError(error)}
function showError(error){globalThis.previewError=error.message;parent.postMessage({type:"postbox-pdf-preview",ready:false,error:String(error.message)},"*");status.dataset.failed='true';status.textContent='PDF preview unavailable. Save the original attachment to open it in a compatible PDF reader.';more.hidden=true}
