(function () {
  const tocLinks = Array.from(document.querySelectorAll('.learn-toc a[data-chapter]'));
  const chapters = tocLinks
    .map((link) => document.getElementById(link.dataset.chapter))
    .filter(Boolean);

  if (!('IntersectionObserver' in window) || chapters.length === 0) {
    return;
  }

  const byId = new Map(tocLinks.map((link) => [link.dataset.chapter, link]));

  const observer = new IntersectionObserver(
    (entries) => {
      const visible = entries
        .filter((entry) => entry.isIntersecting)
        .sort((a, b) => b.intersectionRatio - a.intersectionRatio);
      if (visible.length === 0) return;
      const activeId = visible[0].target.id;
      tocLinks.forEach((link) => link.classList.remove('active'));
      const activeLink = byId.get(activeId);
      if (activeLink) activeLink.classList.add('active');
    },
    {
      rootMargin: '-40% 0px -55% 0px',
      threshold: [0, 0.25, 0.5, 0.75, 1],
    }
  );

  chapters.forEach((chapter) => observer.observe(chapter));

  document.querySelectorAll('.codecard').forEach((card) => {
    const pre = card.querySelector('pre');
    if (!pre) return;
    const shouldCollapse = pre.scrollHeight > 260;
    if (!shouldCollapse) return;

    card.classList.add('collapsed');
    const caption = card.querySelector('figcaption');
    if (!caption) return;
    const toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.className = 'codecard__toggle';
    toggle.textContent = '展开全部';
    caption.appendChild(toggle);
    toggle.addEventListener('click', () => {
      const collapsed = card.classList.toggle('collapsed');
      toggle.textContent = collapsed ? '展开全部' : '收起';
    });
  });
})();
