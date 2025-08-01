pub fn length(string str) -> int:
    $raw (
        return strlen(str);
    )

pub fn compare(string a, string b) -> int:
    $raw (
        return strcmp(a, b);
    )

pub fn concat(string a, string b) -> string:
    $raw (
        char* result = malloc(strlen(a) + strlen(b) + 1);
        strcpy(result, a);
        strcat(result, b);
        return result;
    )
