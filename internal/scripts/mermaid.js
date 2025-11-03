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
      
      // Create new div for mermaid
      const div = document.createElement('div');
      div.className = 'mermaid';
      div.textContent = code;
      
      // Replace pre with div
      pre.replaceWith(div);
    });
    
    // Render all mermaid diagrams
    mermaid.run();
  });
</script>

