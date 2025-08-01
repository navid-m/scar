pub float PI = 3.14159265359
pub float E = 2.71828182846
pub int MAX_INT = 2147483647
pub int MIN_INT = -2147483648

pub fn to_int(string value) -> int:
    int i = 0
    int result = 0
    int sign = 1
    int len = len(value)

    while i < len && (value[i] == ' ' || value[i] == '\t' || value[i] == '\n'):
        reassign i = i + 1
    if i < len  && value[i] == '-':
        reassign sign = -1
        reassign i = i + 1
    elif i < len  && value[i] == '+':
        reassign i = i + 1
    while i < len  && value[i] >= '0'  && value[i] <= '9':
        reassign result = result * 10 + (ord(value[i]) - ord('0'))
        reassign i = i + 1

    return result * sign

pub fn to_float(string value) -> float:
    int i = 0
    float result = 0.0
    int sign = 1
    float fraction = 0.0
    float divisor = 10.0
    int len = len(value)

    while i < len  && (value[i] == ' ' || value[i] == '\t' || value[i] == '\n'):
        reassign i = i + 1
        if i < len  && value[i] == '-':
            reassign sign = -1
            reassign i = i + 1
        elif i < len  && value[i] == '+':
            reassign i = i + 1

    while i < len && value[i] >= '0' && value[i] <= '9':
        reassign result = result * 10.0 + float(ord(value[i]) - ord('0'))
        reassign i = i + 1

    if i < len && value[i] == '.':
        reassign i = i + 1
        while i < len  && value[i] >= '0'  && value[i] <= '9':
            reassign fraction = fraction + (float(ord(value[i]) - ord('0')) / divisor)
            reassign divisor = divisor * 10.0
            reassign i = i + 1

    return float(sign) * (result + fraction)

pub fn min(int a, int b) -> int:
    if a < b:
        return a
    return b

pub fn max(int a, int b) -> int:
    if a > b:
        return a
    return b

pub fn abs(float value) -> float:
    if value < 0.0:
        return -value
    return value
