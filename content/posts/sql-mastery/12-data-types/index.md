# 데이터 타입과 저장

컬럼의 데이터 타입은 저장 크기, 비교 방식, 인덱스 효율, 쿼리 성능에 직접적인 영향을 준다. "동작하면 된다"는 태도로 선택한 타입이 서비스 규모가 커진 뒤에 병목이 되는 경우가 많다. InnoDB가 각 타입을 어떻게 저장하는지 이해하면 올바른 선택이 가능하다.

## 정수 타입

| 타입 | 저장 크기 | 부호 있는 범위 | 부호 없는 범위 |
|---|---|---|---|
| TINYINT | 1바이트 | -128 ~ 127 | 0 ~ 255 |
| SMALLINT | 2바이트 | -32,768 ~ 32,767 | 0 ~ 65,535 |
| MEDIUMINT | 3바이트 | -8,388,608 ~ 8,388,607 | 0 ~ 16,777,215 |
| INT | 4바이트 | -2,147,483,648 ~ 2,147,483,647 | 0 ~ 4,294,967,295 |
| BIGINT | 8바이트 | -9.2 x 10^18 ~ 9.2 x 10^18 | 0 ~ 1.8 x 10^19 |

저장 크기는 고정이다. `INT(11)`의 11은 저장 크기나 범위와 무관하다. 이 숫자는 `ZEROFILL` 옵션과 함께 사용할 때 표시 너비를 지정하는 것이었는데, MySQL 8.0.17부터 display width 자체가 deprecated되었다. `INT`와 `INT(11)`은 동일하다.

### 선택 기준

필요한 범위에 맞는 가장 작은 타입을 선택한다.

- 상태 코드, 플래그: `TINYINT` (1바이트)
- 나이, 수량 등 작은 숫자: `SMALLINT` (2바이트)
- 일반적인 id: `INT UNSIGNED` (4바이트, 약 43억)
- 대규모 테이블의 id, 타임스탬프 기반 값: `BIGINT` (8바이트)

AUTO_INCREMENT PK로 `INT UNSIGNED`를 사용하면 약 43억 행까지 가능하다. 하루 10만 행을 삽입해도 117년이 걸린다. 그래도 확장성이 중요한 시스템에서는 `BIGINT UNSIGNED`를 사용한다.

행 하나에서 INT와 BIGINT의 차이는 4바이트에 불과하지만, 1억 행이면 400MB 차이다. 인덱스까지 포함하면 차이가 더 커진다. clustered index의 PK는 모든 secondary index에도 포함되므로, PK 크기의 영향은 배로 증가한다.

## DECIMAL, FLOAT, DOUBLE

### FLOAT과 DOUBLE

```sql
CREATE TABLE measurements (
    id INT PRIMARY KEY,
    value_float FLOAT,       -- 4바이트, 유효숫자 약 7자리
    value_double DOUBLE      -- 8바이트, 유효숫자 약 15자리
);

INSERT INTO measurements VALUES (1, 0.1 + 0.2, 0.1 + 0.2);
SELECT * FROM measurements;
```

```
+----+--------------------+--------------------+
| id | value_float        | value_double       |
+----+--------------------+--------------------+
|  1 | 0.300000011920929  | 0.30000000000000004|
+----+--------------------+--------------------+
```

FLOAT과 DOUBLE은 IEEE 754 부동소수점 표현을 사용한다. 0.1을 이진수로 정확히 표현할 수 없으므로 근사값이 저장된다. 과학적 계산, 통계 등 근사값이 허용되는 경우에 적합하다.

### DECIMAL

```sql
CREATE TABLE prices (
    id INT PRIMARY KEY,
    price DECIMAL(10, 2)    -- 총 10자리, 소수점 이하 2자리
);

INSERT INTO prices VALUES (1, 0.10 + 0.20);
SELECT * FROM prices;
```

```
+----+-------+
| id | price |
+----+-------+
|  1 |  0.30 |
+----+-------+
```

DECIMAL은 고정 소수점이다. 십진수를 그대로 저장한다. 금액, 환율 등 정확한 소수점 연산이 필요한 곳에 사용한다.

DECIMAL의 저장 크기는 자릿수에 따라 결정된다. 9자리마다 4바이트를 사용한다. `DECIMAL(10, 2)`는 정수부 8자리(4바이트) + 소수부 2자리(1바이트) = 5바이트다. FLOAT(4바이트)보다 크지만, 정확성이 필요한 곳에서는 DECIMAL이 유일한 선택이다.

## 문자열 타입: CHAR vs VARCHAR

### CHAR

```sql
CREATE TABLE country_codes (
    code CHAR(2) NOT NULL  -- 고정 2바이트 (latin1 기준)
);
```

CHAR(N)은 항상 N개 문자 분량의 공간을 사용한다. 'KR'을 저장하면 2바이트, 'A'를 저장해도 2바이트(오른쪽에 공백이 채워진다). 조회 시 후행 공백은 제거된다.

CHAR의 장점은 행 크기가 고정된다는 것이다. 고정 길이 행은 위치 계산이 단순하므로 메모리 관리에서 약간의 이점이 있다. 국가 코드(2자리), 통화 코드(3자리), Y/N 플래그(1자리)처럼 길이가 일정한 데이터에 적합하다.

### VARCHAR

```sql
CREATE TABLE users (
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL
);
```

VARCHAR(N)은 실제 저장된 데이터 길이만큼의 공간을 사용한다. 추가로 길이를 기록하는 1~2바이트가 붙는다. 최대 길이가 255 이하면 1바이트, 256 이상이면 2바이트다.

'Alice'를 VARCHAR(100)에 저장하면 5바이트(데이터) + 1바이트(길이) = 6바이트다. CHAR(100)에 저장하면 100바이트다. 길이가 가변적인 데이터에서는 VARCHAR가 공간 효율적이다.

### VARCHAR(N)의 N은 최대 바이트가 아니라 최대 문자 수다

```sql
-- utf8mb4에서 한글은 문자당 3바이트
-- VARCHAR(100)은 최대 100문자 = 최대 400바이트(utf8mb4 기준)
```

N을 너무 크게 잡으면 문제가 생긴다. MySQL은 메모리 할당 시(임시 테이블, 정렬 버퍼) VARCHAR의 최대 길이를 기준으로 공간을 잡는 경우가 있다. VARCHAR(10000)과 VARCHAR(100)은 디스크에서는 같은 공간을 사용하지만, 메모리에서는 차이가 날 수 있다. 실제 들어올 데이터의 최대 길이에 맞춰 설정한다.

## TEXT vs VARCHAR

VARCHAR의 최대 길이는 행 전체 크기 제한(약 65,535바이트)에 포함된다. 하나의 행에 VARCHAR(10000) 컬럼이 여러 개 있으면 한계에 도달할 수 있다.

TEXT는 이 제한에서 자유롭다. TEXT 데이터는 행 내부에 직접 저장되지 않고, 별도의 외부 페이지(overflow page)에 저장될 수 있다. 행에는 포인터만 남는다.

| 타입 | 최대 크기 |
|---|---|
| TINYTEXT | 255바이트 |
| TEXT | 65,535바이트 (약 64KB) |
| MEDIUMTEXT | 16,777,215바이트 (약 16MB) |
| LONGTEXT | 4,294,967,295바이트 (약 4GB) |

### 언제 TEXT를 쓰는가

- 본문, 설명 등 길이를 예측할 수 없는 긴 텍스트: TEXT
- 이름, 이메일, URL 등 길이가 제한되는 텍스트: VARCHAR

TEXT의 단점:

- 기본값을 지정할 수 없다 (MySQL 8.0.13부터 가능하지만 expression default만).
- TEXT 컬럼에 인덱스를 걸려면 prefix 길이를 지정해야 한다: `CREATE INDEX idx ON posts(content(100))`.
- 정렬 시 `max_sort_length` (기본 1024바이트)까지만 사용된다.

VARCHAR(5000)과 TEXT 중에서 고민된다면, 인덱스가 필요 없고 기본값도 필요 없는 경우 TEXT를 선택한다. 그 외에는 VARCHAR가 더 유연하다.

## DATETIME vs TIMESTAMP

둘 다 날짜와 시간을 저장하지만 내부 표현이 다르다.

| 항목 | DATETIME | TIMESTAMP |
|---|---|---|
| 저장 크기 | 5바이트 | 4바이트 |
| 범위 | 1000-01-01 ~ 9999-12-31 | 1970-01-01 ~ 2038-01-19 |
| 시간대 | 입력값 그대로 저장 | UTC로 변환하여 저장 |

### TIMESTAMP의 시간대 변환

```sql
SET time_zone = '+09:00';
CREATE TABLE events (
    ts TIMESTAMP,
    dt DATETIME
);

INSERT INTO events VALUES (NOW(), NOW());

SELECT * FROM events;
-- ts: 2024-03-15 15:00:00, dt: 2024-03-15 15:00:00

SET time_zone = '+00:00';
SELECT * FROM events;
-- ts: 2024-03-15 06:00:00, dt: 2024-03-15 15:00:00
```

TIMESTAMP는 저장 시 UTC로 변환하고, 조회 시 세션의 time_zone으로 변환한다. time_zone을 바꾸면 표시되는 시간이 달라진다. DATETIME은 입력된 값을 그대로 저장하므로 time_zone 설정과 무관하다.

### 2038년 문제

TIMESTAMP는 내부적으로 Unix timestamp(32비트 정수)로 저장된다. 2038-01-19 03:14:07 UTC에 오버플로가 발생한다. 신규 테이블에서는 DATETIME을 사용하는 것이 안전하다.

### 소수점 이하 초

MySQL 5.6.4부터 소수점 이하 초를 지원한다.

```sql
CREATE TABLE logs (
    created_at DATETIME(3)  -- 밀리초까지 저장. 추가 2바이트
);

-- DATETIME(0): 5바이트 (기본)
-- DATETIME(3): 5 + 2 = 7바이트
-- DATETIME(6): 5 + 3 = 8바이트 (마이크로초)
```

## ENUM과 SET

### ENUM

```sql
CREATE TABLE shirts (
    size ENUM('S', 'M', 'L', 'XL') NOT NULL
);

INSERT INTO shirts VALUES ('M');
INSERT INTO shirts VALUES ('XXL');  -- 에러: 허용되지 않는 값
```

ENUM은 내부적으로 정수로 저장된다. 'S' = 1, 'M' = 2, 'L' = 3, 'XL' = 4. 저장 크기는 멤버 수에 따라 1~2바이트다(255개 이하면 1바이트, 65,535개 이하면 2바이트).

문자열 'M'을 VARCHAR(2)로 저장하면 3바이트(2 + 길이 1바이트)다. ENUM이면 1바이트다. 행 수가 많으면 차이가 누적된다.

ENUM의 문제점:

- **변경 비용이 크다**: 새 값을 추가하려면 ALTER TABLE이 필요하다. MySQL 8.0에서 ENUM 목록 끝에 값을 추가하는 것은 메타데이터만 수정하므로 빠르지만(INSTANT), 중간에 삽입하거나 순서를 변경하면 테이블 재구성이 필요하다.
- **정렬이 문자열 순서가 아니다**: ENUM은 내부 정수값 순서로 정렬된다. 'S', 'M', 'L', 'XL' 순서는 정의 순서이지 알파벳 순서가 아니다.
- **숫자 문자열 혼동**: `ENUM('0', '1', '2')`에서 `INSERT INTO t VALUES (1)`은 '0'이 들어간다(인덱스 1번). 문자열 '1'을 넣으려면 `VALUES ('1')`로 따옴표를 써야 한다.

상태값이 고정적이고 변경 가능성이 낮으면 ENUM이 유리하다. 빈번하게 변경되는 값이면 VARCHAR + CHECK 제약조건이나 참조 테이블을 사용한다.

### SET

SET은 ENUM과 비슷하지만 여러 값을 동시에 저장할 수 있다.

```sql
CREATE TABLE user_preferences (
    notifications SET('email', 'sms', 'push') NOT NULL
);

INSERT INTO user_preferences VALUES ('email,push');
SELECT * FROM user_preferences WHERE FIND_IN_SET('email', notifications);
```

SET은 비트마스크로 저장된다. 'email' = 1, 'sms' = 2, 'push' = 4. 'email,push'는 1 + 4 = 5로 저장된다. 최대 64개 멤버까지 가능하다. 실무에서는 정규화된 별도 테이블이 더 유연하므로 SET의 사용 빈도는 낮다.

## 문자셋과 콜레이션

### 문자셋 (charset)

문자셋은 문자를 바이트로 인코딩하는 방식이다. MySQL에서 주로 사용하는 문자셋:

- **latin1**: 1바이트. 서유럽 언어만 지원한다.
- **utf8** (utf8mb3): 최대 3바이트. BMP(Basic Multilingual Plane)만 지원한다.
- **utf8mb4**: 최대 4바이트. 모든 유니코드를 지원한다.

### 콜레이션 (collation)

콜레이션은 문자열을 비교하고 정렬하는 규칙이다.

```sql
-- utf8mb4_general_ci: 대소문자 무시, 악센트 처리는 비일관적 (정확한 accent insensitive가 필요하면 _ai 접미사 콜레이션 사용)
-- utf8mb4_unicode_ci: 유니코드 규칙, 더 정확한 비교
-- utf8mb4_bin: 바이트 단위 비교
-- utf8mb4_0900_ai_ci: MySQL 8.0 기본, UCA 9.0.0 기반

SELECT 'A' = 'a' COLLATE utf8mb4_general_ci;  -- 1 (같다)
SELECT 'A' = 'a' COLLATE utf8mb4_bin;          -- 0 (다르다)
```

콜레이션 이름의 접미사:

- `_ci`: Case Insensitive. 대소문자를 구분하지 않는다.
- `_cs`: Case Sensitive. 대소문자를 구분한다.
- `_bin`: Binary. 바이트 값으로 비교한다.
- `_ai`: Accent Insensitive. 악센트를 구분하지 않는다.

콜레이션은 WHERE 조건, ORDER BY, 유니크 인덱스에 영향을 준다.

```sql
-- utf8mb4_general_ci에서는 'Alice'와 'alice'가 같다
CREATE TABLE users (
    email VARCHAR(255) UNIQUE
) COLLATE utf8mb4_general_ci;

INSERT INTO users VALUES ('alice@example.com');
INSERT INTO users VALUES ('Alice@example.com');  -- Duplicate entry 에러
```

대소문자를 구분하는 유니크 제약이 필요하면 `utf8mb4_bin`이나 `utf8mb4_0900_as_cs`를 사용한다.

### JOIN 시 콜레이션 불일치

```sql
-- table_a: utf8mb4_general_ci
-- table_b: utf8mb4_unicode_ci
SELECT * FROM table_a a
JOIN table_b b ON a.name = b.name;
-- 경고: Illegal mix of collations
```

JOIN이나 비교 연산에서 양쪽 컬럼의 콜레이션이 다르면 에러 또는 암묵적 변환이 발생한다. 암묵적 변환이 일어나면 인덱스를 사용하지 못한다. 테이블 간 콜레이션을 통일하는 것이 중요하다.

## utf8 vs utf8mb4

MySQL의 `utf8`은 진짜 UTF-8이 아니다. MySQL은 초기 구현에서 UTF-8을 최대 3바이트로 제한했다. 이 3바이트 UTF-8을 `utf8`(정식 명칭 `utf8mb3`)이라 부른다.

UTF-8 표준은 최대 4바이트다. 4바이트가 필요한 문자:

- 이모지 (U+1F600 등)
- 일부 한자 (CJK Extension B 이후)
- 음악 기호, 수학 기호 일부

`utf8mb4`가 진짜 UTF-8이다.

```sql
-- utf8 (utf8mb3): 이모지 저장 불가
CREATE TABLE test_utf8 (content VARCHAR(100)) CHARSET utf8;
INSERT INTO test_utf8 VALUES ('hello');  -- OK

-- utf8mb4: 모든 유니코드 저장 가능
CREATE TABLE test_utf8mb4 (content VARCHAR(100)) CHARSET utf8mb4;
INSERT INTO test_utf8mb4 VALUES ('hello');  -- OK
```

MySQL 8.0부터 기본 문자셋이 `utf8mb4`다. 신규 테이블은 별도 설정 없이 utf8mb4가 적용된다. 레거시 시스템에서 utf8(utf8mb3)을 사용하고 있다면 utf8mb4로 마이그레이션해야 한다.

마이그레이션 시 주의: utf8mb4는 문자당 최대 4바이트이므로, 기존 utf8의 VARCHAR(255)가 utf8mb4로 변환되면 인덱스 크기 제한(기본 3072바이트)에 걸릴 수 있다. utf8에서 VARCHAR(255)는 765바이트지만, utf8mb4에서는 1020바이트다.

## 잘못된 타입 선택이 만드는 문제

### IP 주소를 VARCHAR에 저장

```sql
-- 비효율적: 15바이트 + 길이 바이트
ip_address VARCHAR(15)

-- 효율적: 4바이트 (IPv4)
ip_address INT UNSIGNED
-- INET_ATON('192.168.1.1') -> 3232235777
-- INET_NTOA(3232235777) -> '192.168.1.1'
```

IPv4 주소는 4바이트 정수로 저장하면 공간이 1/4로 줄고, 비교와 범위 검색도 빨라진다.

### 날짜를 VARCHAR에 저장

```sql
-- 나쁘다: 10바이트 + 길이 바이트, 비교 시 문자열 정렬
created_date VARCHAR(10)  -- '2024-03-15'

-- 좋다: 3바이트, 날짜 연산 가능
created_date DATE
```

VARCHAR로 저장하면 날짜 함수(`DATE_ADD`, `DATEDIFF` 등)를 사용할 수 없고, 범위 검색에서 인덱스 효율이 떨어진다. `'2024-03-15' > '2024-3-9'`는 문자열 비교에서 TRUE인데, 날짜로는 올바른 비교지만 `'2024-3-9'` 형식이면 결과가 달라질 수 있다.

### BIGINT에 짧은 문자열 코드 저장

```sql
-- 비효율적: 8바이트
status BIGINT  -- 1 = active, 2 = inactive

-- 효율적: 1바이트
status TINYINT  -- 1 = active, 2 = inactive
```

상태값이 10개 이하인데 BIGINT을 사용하면 행당 7바이트를 낭비한다.

### BOOLEAN

MySQL에는 별도의 BOOLEAN 타입이 없다. `BOOLEAN`은 `TINYINT(1)`의 alias다.

```sql
CREATE TABLE settings (
    is_active BOOLEAN DEFAULT TRUE
);
-- 내부적으로 TINYINT(1)로 생성된다
-- TRUE = 1, FALSE = 0
```

1바이트로 0 또는 1을 저장한다. BIT(1)이 1비트만 사용할 것 같지만, InnoDB는 BIT 컬럼도 최소 1바이트를 할당하므로 공간 차이는 없다.

데이터 타입 선택은 테이블 설계의 기초다. 행 하나의 차이는 작지만, 수억 행이 쌓이고 인덱스가 복제되면 그 차이가 GB 단위로 누적된다. 가장 작은 타입, 가장 정확한 타입을 선택한다.
