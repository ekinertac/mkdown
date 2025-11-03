<script type="module">
  import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
  
  mermaid.initialize({ 
    startOnLoad: false,
    theme: '{{THEME}}',
    securityLevel: 'loose'
  });

  // Convert code blocks with language-mermaid to mermaid divs
  document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('code.language-mermaid').forEach((block, index) => {
      const pre = block.parentElement;
      const code = block.textContent;
      
      // Create wrapper for diagram and fullscreen button
      const wrapper = document.createElement('div');
      wrapper.className = 'mermaid-wrapper';
      
      // Create mermaid div
      const div = document.createElement('div');
      div.className = 'mermaid';
      div.textContent = code;
      
      // Create fullscreen button
      const fsBtn = document.createElement('button');
      fsBtn.className = 'mermaid-fullscreen-btn';
      fsBtn.innerHTML = '⛶';
      fsBtn.title = 'Toggle fullscreen';
      fsBtn.setAttribute('aria-label', 'Toggle fullscreen');
      
      wrapper.appendChild(div);
      wrapper.appendChild(fsBtn);
      
      // Replace pre with wrapper
      pre.replaceWith(wrapper);
      
      // Add fullscreen toggle
      fsBtn.addEventListener('click', () => {
        wrapper.classList.toggle('fullscreen');
        fsBtn.innerHTML = wrapper.classList.contains('fullscreen') ? '✕' : '⛶';
      });
    });
    
    // Render all mermaid diagrams
    mermaid.run();
    
    // Close fullscreen with Escape
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        document.querySelectorAll('.mermaid-wrapper.fullscreen').forEach(w => {
          w.classList.remove('fullscreen');
          w.querySelector('.mermaid-fullscreen-btn').innerHTML = '⛶';
        });
      }
    });
  });
</script>

