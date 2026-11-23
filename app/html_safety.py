"""Conservative mail CSS filtering; sandbox/CSP remain separate defenses."""
import re


def safe_css(value):
    # Do not preserve escaped or comment-obfuscated network/active CSS. This
    # deliberately drops the whole affected style rather than rewriting a URL.
    plain = re.sub(r'/\*.*?\*/', '', value, flags=re.S)
    if ('\\' in plain or '<' in plain or '>' in plain or
            re.search(r'url\s*\(|(?:-webkit-)?image-set\s*\(|expression\s*\(|@import|@font-face|(?:behavior|-moz-binding)\s*:|https?:|//', plain, re.I)):
        return ''
    return plain
