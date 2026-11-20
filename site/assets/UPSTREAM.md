# Upstream

`site.css`, `site.js` and `fonts/` come from [math2-agent](https://github.com/tamnd/math2-agent) at commit `a82a3ddc7a828fed3258dee146c0c7249b3747d2` (`site/assets`), under Apache-2.0. The fonts keep their own licences in `fonts/`.

We keep `site.css` without change, so the two sites look the same. The style of riemann.tamnd.com that math2 does not have is in `riemann.css`, which loads after `site.css`.

We changed two status texts of the search in `site.js`. To take a new version, copy the files again and apply the same two changes.
