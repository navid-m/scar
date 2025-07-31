pub float PI = 3.14159265359
pub float E = 2.71828182846
pub int MAX_INT = 2147483647
pub int MIN_INT = -2147483648

pub fn to_int(string value) -> int:
    $raw (
        return atoi(value);
    )

pub fn to_float(string value) -> float:
    $raw (
        return atof(value);
    )

pub fn min(int a, int b) -> int:
    if a < b:
        return a
    return b

pub fn max(int a, int b) -> int:
    if a > b:
        return a
    return b

pub fn abs(int value) -> int:
    if value < 0.0:
        return -value
    return value
