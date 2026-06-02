(function () {
  function init() {
    var popup = document.getElementById('wechat-qr-popup');
    if (!popup) {
      popup = document.createElement('div');
      popup.id = 'wechat-qr-popup';
      popup.innerHTML = '<img src="/images/wechat.png" alt="WeChat QR"><p>扫码加我微信</p>';
      document.body.appendChild(popup);
    }

    var links = document.querySelectorAll('a');
    for (var i = 0; i < links.length; i++) {
      (function (a) {
        if (!a.querySelector('.fa-weixin')) return;
        if (a.dataset.wechatBound) return;
        a.dataset.wechatBound = '1';

        a.addEventListener('mouseenter', function () {
          var rect = a.getBoundingClientRect();
          popup.style.display = 'block';
          var popupWidth = popup.offsetWidth;
          var left = rect.left + rect.width / 2 - popupWidth / 2;
          left = Math.max(8, Math.min(left, window.innerWidth - popupWidth - 8));
          popup.style.left = left + 'px';
          popup.style.top = rect.bottom + 10 + 'px';
        });

        a.addEventListener('mouseleave', function () {
          popup.style.display = 'none';
        });

        a.addEventListener('click', function (e) {
          e.preventDefault();
        });
      })(links[i]);
    }
  }

  if (document.readyState !== 'loading') {
    init();
  } else {
    document.addEventListener('DOMContentLoaded', init);
  }
})();
