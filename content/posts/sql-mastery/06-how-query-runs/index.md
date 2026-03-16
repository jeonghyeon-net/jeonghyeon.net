# 쿼리가 실행되기까지

`SELECT * FROM orders WHERE customer_id = 42`을 실행하면, 결과가 돌아오기까지 내부에서 여러 단계를 거친다. 텍스트를 파싱하고, 의미를 검증하고, 최적의 실행 방법을 결정하고, 실제로 데이터를 읽는다. 이 과정을 이해하면 쿼리가 왜 느린지, 어떻게 개선해야 하는지를 판단할 수 있게 된다.

## SQL 실행의 전체 흐름

MySQL에서 쿼리가 실행되는 과정은 크게 네 단계로 나뉜다:

```
SQL 텍스트
    |
    v
[파서 (Parser)]          → 문법 검사, 파스 트리 생성
    |
    v
[전처리기 (Preprocessor)] → 의미 검증 (테이블/컬럼 존재 여부, 권한)
    |
    v
[옵티마이저 (Optimizer)]  → 실행 계획 결정
    |
    v
[실행기 (Executor)]       → 스토리지 엔진을 호출하여 데이터를 읽고 결과 반환
```

### 파서 (Parser)

SQL 텍스트를 입력으로 받아, 문법적으로 올바른지 검사하고, 내부 자료구조인 파스 트리(parse tree)로 변환한다.

```sql
-- 문법 오류: FROM이 빠졌다
SELECT * WHERE id = 1;
-- ERROR 1064 (42000): You have an error in your SQL syntax
```

파서는 SQL의 구조(키워드, 테이블명, 컬럼명, 조건식 등)를 분석할 뿐, 해당 테이블이나 컬럼이 실제로 존재하는지는 확인하지 않는다. 그것은 다음 단계의 역할이다.

### 전처리기 (Preprocessor)

파스 트리를 받아 의미적 유효성을 검증한다:

- 참조하는 테이블이 존재하는가
- 참조하는 컬럼이 해당 테이블에 존재하는가
- 현재 사용자가 해당 테이블에 접근할 권한이 있는가
- alias가 모호하지 않은가

```sql
-- 테이블은 존재하지만 컬럼이 없다
SELECT nonexistent_column FROM orders;
-- ERROR 1054 (42S22): Unknown column 'nonexistent_column' in 'field list'
```

문법 오류(파서)와 의미 오류(전처리기)는 에러 코드가 다르다. 문법 오류는 1064, 존재하지 않는 컬럼은 1054다.

### 옵티마이저 (Optimizer)

전처리를 통과한 쿼리에 대해, 가능한 실행 방법 중 비용이 가장 낮은 것을 선택한다. 이 단계의 결과물이 실행 계획(execution plan)이다.

같은 결과를 반환하는 쿼리라도 실행 방법은 여러 가지가 있다:

```sql
SELECT * FROM orders WHERE customer_id = 42 AND status = 'pending';
```

가능한 실행 방법:
- 풀 테이블 스캔 후 두 조건으로 필터링
- `idx_customer` 인덱스로 `customer_id = 42`를 찾고, 결과에서 `status` 필터링
- `idx_status` 인덱스로 `status = 'pending'`을 찾고, 결과에서 `customer_id` 필터링
- `idx_customer_status` 복합 인덱스로 두 조건을 동시에 만족하는 행을 찾기

옵티마이저는 각 방법의 비용을 추정하고, 가장 저렴한 것을 선택한다.

### 실행기 (Executor)

옵티마이저가 결정한 실행 계획에 따라, 스토리지 엔진의 API를 호출하여 데이터를 읽는다. InnoDB의 경우 buffer pool에서 page를 읽고, 필요하면 디스크에서 page를 가져온다. 실행기는 스토리지 엔진이 반환한 행들을 모아서 클라이언트에 전달한다.

## 옵티마이저와 비용 기반 최적화

MySQL 옵티마이저는 비용 기반 최적화(Cost-Based Optimization, CBO)를 수행한다. "비용"이란 쿼리를 실행하는 데 필요한 자원의 추정치다. 주로 디스크 I/O 횟수와 CPU 연산량을 기준으로 산정한다.

### 비용 계산의 요소

옵티마이저가 비용을 추정할 때 고려하는 주요 요소:

- **page 읽기 횟수**: 디스크에서 읽어야 하는 page의 수. buffer pool에 있으면 비용이 낮고, 디스크에서 읽어야 하면 비용이 높다.
- **행 비교 횟수**: 조건을 평가하기 위해 비교해야 하는 행의 수.
- **정렬 비용**: `ORDER BY`가 있을 때, 인덱스 순서와 맞지 않으면 filesort가 필요하다.
- **임시 테이블 생성 비용**: `GROUP BY`나 `DISTINCT` 등에서 임시 테이블이 필요할 수 있다.

MySQL 8.0에서는 `mysql.server_cost`와 `mysql.engine_cost` 테이블에 각 연산의 단위 비용이 정의되어 있다:

```sql
SELECT * FROM mysql.server_cost;
```

```
+------------------------------+------------+
| cost_name                    | cost_value |
+------------------------------+------------+
| disk_temptable_create_cost   |       NULL |
| disk_temptable_row_cost      |       NULL |
| key_compare_cost             |       NULL |
| memory_temptable_create_cost |       NULL |
| memory_temptable_row_cost    |       NULL |
| row_evaluate_cost            |       NULL |
+------------------------------+------------+
```

`NULL`은 기본값을 사용한다는 뜻이다. 기본값은 소스 코드에 하드코딩되어 있다. 예를 들어 `row_evaluate_cost`의 기본값은 0.1이다.

### 옵티마이저의 한계

비용 기반 최적화는 통계에 의존한다. 통계가 부정확하면 잘못된 실행 계획을 선택한다. 몇 가지 전형적인 상황:

- **데이터 분포의 편향**: `status` 컬럼에서 'pending'이 1%, 'delivered'가 90%를 차지한다. 하지만 통계는 균등 분포를 가정할 수 있다. 이 경우 옵티마이저가 `status = 'pending'`을 검색할 때 실제보다 많은 행이 매칭될 것으로 잘못 추정하여, 인덱스 대신 풀 스캔을 선택할 수 있다.
- **상관관계가 있는 컬럼**: `city = '서울'`인 행의 대부분이 `country = '한국'`이다. 하지만 옵티마이저는 각 컬럼의 통계를 독립적으로 다룬다. 두 조건의 결합 선택도를 실제보다 낮게 추정할 수 있다.
- **오래된 통계**: 대량의 INSERT나 DELETE 이후 통계가 갱신되지 않은 경우.

이런 상황에서 `ANALYZE TABLE`로 통계를 갱신하거나, 인덱스 힌트(05편 참고)를 사용하여 옵티마이저의 선택을 보정할 수 있다.

## 통계 정보

옵티마이저의 판단 근거가 되는 통계 정보는 크게 두 가지다.

### 인덱스 cardinality

05편에서 다룬 cardinality. 인덱스 컬럼의 고유 값 수 추정치다. InnoDB는 인덱스의 page를 무작위로 샘플링하여 계산한다. 샘플 page 수는 `innodb_stats_persistent_sample_pages` 설정으로 조절한다. 기본값은 20이다.

```sql
SHOW VARIABLES LIKE 'innodb_stats_persistent_sample_pages';
```

```
+--------------------------------------+-------+
| Variable_name                        | Value |
+--------------------------------------+-------+
| innodb_stats_persistent_sample_pages | 20    |
+--------------------------------------+-------+
```

샘플 수를 늘리면 통계가 정확해지지만, `ANALYZE TABLE` 실행 시간이 길어진다.

### 테이블 행 수 추정치

InnoDB에서 테이블의 정확한 행 수를 알려면 전체 테이블을 스캔해야 한다. MVCC 구조 때문에 트랜잭션마다 보이는 행의 수가 다를 수 있기 때문이다. 따라서 `SHOW TABLE STATUS`나 `information_schema.TABLES`에 표시되는 행 수는 추정치다:

```sql
SHOW TABLE STATUS LIKE 'orders'\G
```

```
           Name: orders
           Rows: 987654
 Avg_row_length: 198
    Data_length: 195624960
```

`Rows`가 100만이 아니라 987,654로 표시될 수 있다. 추정치이기 때문이다. 정확한 행 수가 필요하면 `SELECT COUNT(*) FROM orders`를 실행해야 한다. 하지만 이 쿼리 자체가 풀 스캔을 유발한다.

## 실행 계획

옵티마이저가 결정한 쿼리 실행 방법을 실행 계획(execution plan)이라고 한다. 쿼리를 최적화할 때 가장 먼저 확인해야 하는 것이 실행 계획이다.

실행 계획은 쿼리 앞에 `EXPLAIN`을 붙여서 확인한다. 쿼리를 실제로 실행하지 않고, 어떻게 실행할 것인지를 보여 준다.

## EXPLAIN 기초

```sql
EXPLAIN SELECT * FROM orders WHERE customer_id = 42;
```

```
+----+-------------+--------+------+---------------+--------------+---------+-------+------+-------+
| id | select_type | table  | type | possible_keys | key          | key_len | ref   | rows | Extra |
+----+-------------+--------+------+---------------+--------------+---------+-------+------+-------+
|  1 | SIMPLE      | orders | ref  | idx_customer  | idx_customer | 4       | const |   20 | NULL  |
+----+-------------+--------+------+---------------+--------------+---------+-------+------+-------+
```

각 컬럼의 의미를 살펴본다.

### type -- 접근 방식

쿼리가 테이블의 데이터에 어떻게 접근하는지를 나타낸다. 성능 판단에 가장 중요한 컬럼이다. 위에서 아래로 갈수록 나쁘다(읽는 행이 많다):

| type | 설명 |
|---|---|
| `system` | 테이블에 행이 하나뿐. 사실상 상수. |
| `const` | PK 또는 UNIQUE 인덱스로 정확히 한 행을 찾는 경우. |
| `eq_ref` | JOIN에서 PK 또는 UNIQUE 인덱스로 한 행씩 매칭. |
| `ref` | 인덱스를 사용하여 여러 행을 찾는 경우. |
| `range` | 인덱스를 사용한 범위 검색 (`BETWEEN`, `>`, `<`, `IN`). |
| `index` | 인덱스 전체를 스캔 (풀 인덱스 스캔). |
| `ALL` | 테이블 전체를 스캔 (풀 테이블 스캔). |

`ALL`이 보이면 인덱스를 전혀 활용하지 못하고 있다는 뜻이다. 대부분의 경우 개선이 필요하다. `index`도 주의가 필요하다. 인덱스를 사용하긴 하지만, 인덱스의 모든 항목을 순서대로 읽는 것이므로 대량의 데이터에서는 느릴 수 있다.

`const`와 `eq_ref`는 이상적인 상태다. `ref`와 `range`는 대부분의 쿼리에서 충분히 좋은 결과를 낸다.

### key -- 사용된 인덱스

옵티마이저가 실제로 선택한 인덱스의 이름이다. `NULL`이면 인덱스를 사용하지 않았다는 뜻이다. `possible_keys`에는 사용 가능한 인덱스 후보가 표시되고, `key`에는 그중에서 실제로 선택된 것이 표시된다.

```sql
EXPLAIN SELECT * FROM orders WHERE customer_id = 42 AND status = 'pending';
```

```
+------+---------------+--------------+------+------+-------+
| type | possible_keys | key          | ref  | rows | Extra |
+------+---------------+--------------+------+------+-------+
| ref  | idx_cust,idx_stat | idx_cust | const|   20 | Using where |
+------+---------------+--------------+------+------+-------+
```

`possible_keys`에 `idx_cust`와 `idx_stat`가 있지만, 옵티마이저는 `idx_cust`를 선택했다. `status` 조건은 인덱스가 아닌 행 필터링으로 처리된다.

### rows -- 예상 행 수

옵티마이저가 이 단계에서 읽을 것으로 추정하는 행의 수다. 정확한 값이 아니라 추정치다. 이 값이 클수록 비용이 크다.

JOIN이 있는 쿼리에서는 여러 행이 EXPLAIN 결과에 나타나고, 각 행의 `rows`를 곱한 것이 전체 예상 처리량에 가까워진다. `rows`가 1,000과 500인 두 테이블의 JOIN이면, 최악의 경우 500,000번의 비교가 발생한다.

### Extra -- 추가 정보

실행 방식에 대한 부가 정보를 담고 있다. 자주 보이는 값들:

| Extra | 의미 |
|---|---|
| `Using index` | Covering index로 쿼리를 해결. 테이블 데이터 접근 없음. 좋다. |
| `Using where` | 스토리지 엔진에서 가져온 행을 MySQL 서버 레이어에서 추가 필터링. |
| `Using temporary` | 쿼리 처리에 임시 테이블 사용. GROUP BY, DISTINCT 등에서 발생. |
| `Using filesort` | 결과를 정렬하기 위해 추가 정렬 작업 수행. 인덱스 순서와 맞지 않을 때 발생. |
| `Using index condition` | Index condition pushdown. 인덱스 레벨에서 조건을 평가하여 불필요한 행 접근을 줄임. |

`Using temporary`와 `Using filesort`가 동시에 나타나면, 임시 테이블을 만들고 거기서 다시 정렬하는 것이므로 비용이 크다. 인덱스 설계를 재검토해야 할 신호다.

## EXPLAIN으로 실행 계획 읽기

몇 가지 쿼리로 실행 계획을 읽는 연습을 해 보자.

### 예제 1: PK 검색

```sql
EXPLAIN SELECT * FROM orders WHERE id = 1;
```

```
+------+--------+---------+-------+------+-------+
| type | key    | key_len | ref   | rows | Extra |
+------+--------+---------+-------+------+-------+
| const| PRIMARY| 4       | const |    1 | NULL  |
+------+--------+---------+-------+------+-------+
```

- `type = const`: PK로 정확히 한 행을 찾았다. 최고 효율.
- `key = PRIMARY`: clustered index(PK) 사용.
- `rows = 1`: 한 행만 읽는다.

이것이 가장 이상적인 실행 계획이다.

### 예제 2: 인덱스를 사용한 범위 검색

```sql
EXPLAIN SELECT * FROM orders
WHERE customer_id = 42
AND order_date BETWEEN '2024-01-01' AND '2024-06-30';
```

```
+-------+----------------+---------+------+------+-------------+
| type  | key            | key_len | ref  | rows | Extra       |
+-------+----------------+---------+------+------+-------------+
| range | idx_cust_date  | 7       | NULL |   12 | Using where |
+-------+----------------+---------+------+------+-------------+
```

- `type = range`: 인덱스를 사용한 범위 검색.
- `key = idx_cust_date`: `(customer_id, order_date)` 복합 인덱스 사용.
- `rows = 12`: 약 12행을 읽을 것으로 추정.
- `key_len = 7`: 인덱스의 어느 부분까지 사용했는지를 바이트로 표시. `customer_id`(INT, 4바이트) + `order_date`(DATE, 3바이트) = 7바이트. 복합 인덱스의 두 컬럼 모두 활용되었다.

### 예제 3: 풀 테이블 스캔

```sql
EXPLAIN SELECT * FROM orders WHERE YEAR(order_date) = 2024;
```

```
+------+------+---------+------+---------+-------------+
| type | key  | key_len | ref  | rows    | Extra       |
+------+------+---------+------+---------+-------------+
| ALL  | NULL | NULL    | NULL | 1000000 | Using where |
+------+------+---------+------+---------+-------------+
```

- `type = ALL`: 풀 테이블 스캔.
- `key = NULL`: 인덱스를 사용하지 않았다.
- `rows = 1000000`: 테이블의 모든 행을 읽어야 한다.

05편에서 설명한 대로, 컬럼에 함수를 적용하면 인덱스를 사용할 수 없다. 범위 조건으로 바꾸면:

```sql
EXPLAIN SELECT * FROM orders
WHERE order_date >= '2024-01-01' AND order_date < '2025-01-01';
```

```
+-------+--------------+---------+------+------+-------------+
| type  | key          | key_len | ref  | rows | Extra       |
+-------+--------------+---------+------+------+-------------+
| range | idx_date     | 3       | NULL |  365 | Using where |
+-------+--------------+---------+------+------+-------------+
```

`type`이 `ALL`에서 `range`로 바뀌었다. `rows`가 1,000,000에서 365로 줄었다. 같은 결과를 반환하지만 성능 차이는 극명하다.

### 예제 4: Covering index

```sql
EXPLAIN SELECT customer_id, order_date FROM orders WHERE customer_id = 42;
```

```
+------+---------------+---------+-------+------+-------------+
| type | key           | key_len | ref   | rows | Extra       |
+------+---------------+---------+-------+------+-------------+
| ref  | idx_cust_date | 4       | const |   20 | Using index |
+------+---------------+---------+-------+------+-------------+
```

- `Extra = Using index`: covering index. 인덱스만으로 쿼리를 해결했다.

같은 WHERE 조건이라도, SELECT하는 컬럼에 따라 실행 계획이 달라진다. `SELECT *`로 바꾸면 `Using index`가 사라지고 PK lookup이 추가된다.

### EXPLAIN FORMAT=JSON

기본 EXPLAIN보다 더 상세한 정보가 필요하면 JSON 형식을 사용한다:

```sql
EXPLAIN FORMAT=JSON SELECT * FROM orders WHERE customer_id = 42\G
```

JSON 형식에서는 각 단계의 예상 비용(`query_cost`), 실제 사용된 비용 모델의 세부 항목 등을 확인할 수 있다. 옵티마이저가 왜 특정 인덱스를 선택했는지 판단하는 데 도움이 된다.

### EXPLAIN ANALYZE

MySQL 8.0.18부터 지원되는 `EXPLAIN ANALYZE`는 실제로 쿼리를 실행하면서 각 단계의 실측 데이터를 수집한다:

```sql
EXPLAIN ANALYZE SELECT * FROM orders WHERE customer_id = 42;
```

```
-> Index lookup on orders using idx_customer (customer_id=42)
   (cost=6.50 rows=20)
   (actual time=0.087..0.134 rows=18 loops=1)
```

`rows=20`은 옵티마이저의 추정치이고, `actual ... rows=18`은 실제로 읽은 행 수다. 추정치와 실측치의 차이가 크면 통계가 부정확하다는 신호다. 단, `EXPLAIN ANALYZE`는 쿼리를 실제로 실행하므로, 느린 쿼리에 사용할 때는 주의가 필요하다.

## 실행 계획을 읽는 습관

쿼리 성능 문제를 만나면, 가장 먼저 `EXPLAIN`을 실행한다. 다음 순서로 확인한다:

1. **type을 본다**: `ALL`이면 풀 스캔이다. 인덱스가 필요한지 검토한다.
2. **key를 본다**: 의도한 인덱스가 사용되고 있는지 확인한다.
3. **rows를 본다**: 예상 행 수가 합리적인지 확인한다.
4. **Extra를 본다**: `Using filesort`나 `Using temporary`가 있으면 개선 여지가 있다.

이 네 가지만 확인해도 대부분의 쿼리 성능 문제를 진단할 수 있다.
