# 정규화

테이블 설계가 잘못되면 데이터가 중복되고, 중복은 모순을 만든다. 정규화는 이 문제를 체계적으로 해결하는 방법이다. 관계형 데이터베이스 이론에서 가장 오래되고 가장 기본적인 설계 원칙이다.

## 정규화가 필요한 이유

다음 테이블을 보자. 주문과 상품 정보를 하나의 테이블에 담았다:

```sql
CREATE TABLE orders (
    order_id    INT,
    customer    VARCHAR(100),
    product     VARCHAR(100),
    category    VARCHAR(50),
    price       DECIMAL(10,2),
    quantity    INT
);

INSERT INTO orders VALUES
(1, '김철수', '키보드', '주변기기', 89000, 2),
(2, '이영희', '모니터', '디스플레이', 350000, 1),
(3, '김철수', '키보드', '주변기기', 89000, 1),
(4, '박민수', '마우스', '주변기기', 45000, 3);
```

이 테이블에는 세 가지 이상(anomaly)이 존재한다.

### 갱신 이상 (update anomaly)

키보드의 가격이 95,000원으로 변경되었다. `order_id = 1`과 `order_id = 3`에 모두 키보드가 있으므로 두 행을 모두 수정해야 한다:

```sql
UPDATE orders SET price = 95000 WHERE product = '키보드';
```

한 행만 수정하면 같은 상품인데 가격이 다른 모순이 발생한다. 데이터가 중복 저장되어 있기 때문이다.

### 삽입 이상 (insert anomaly)

새로운 상품 "웹캠"을 등록하고 싶다. 하지만 이 테이블에 상품을 넣으려면 주문 정보(`order_id`, `customer`, `quantity`)도 함께 입력해야 한다. 아직 주문이 없는 상품은 등록할 수 없다. 상품 정보가 주문에 종속되어 있기 때문이다.

### 삭제 이상 (delete anomaly)

`order_id = 2`를 삭제하면 이영희의 주문이 사라지는 동시에 "모니터"라는 상품과 "디스플레이"라는 카테고리 정보도 함께 사라진다. 주문을 지우고 싶었을 뿐인데 상품 정보까지 유실된다.

세 가지 이상 모두 하나의 원인에서 비롯된다. 서로 다른 사실(주문, 상품, 카테고리)이 하나의 테이블에 뒤섞여 있다는 것이다. 정규화는 이 뒤섞인 사실들을 분리하는 과정이다.

## 함수 종속

정규화를 이해하려면 함수 종속(functional dependency)을 먼저 알아야 한다.

속성 X의 값이 결정되면 속성 Y의 값이 하나로 결정될 때, Y는 X에 함수 종속된다고 한다. `X -> Y`로 표기한다.

- `order_id -> customer`: 주문 번호가 정해지면 고객이 결정된다.
- `product -> category`: 상품이 정해지면 카테고리가 결정된다.
- `product -> price`: 상품이 정해지면 가격이 결정된다.

함수 종속은 데이터의 의미에서 비롯된다. 테이블에 저장된 현재 데이터를 보고 판단하는 것이 아니라, 비즈니스 규칙에 따라 결정된다.

## 제1정규형 (1NF)

제1정규형의 조건은 모든 컬럼이 원자값(atomic value)을 가지는 것이다. 하나의 셀에 여러 값이 들어가면 안 된다.

위반 사례:

```sql
CREATE TABLE student_courses (
    student_id   INT,
    name         VARCHAR(100),
    courses      VARCHAR(500)   -- '수학,영어,물리' 같은 값
);

INSERT INTO student_courses VALUES
(1, '김철수', '수학,영어,물리'),
(2, '이영희', '영어,화학');
```

`courses` 컬럼에 여러 과목이 쉼표로 구분되어 들어가 있다. "수학을 듣는 학생"을 찾으려면 `LIKE '%수학%'`을 써야 하고, 인덱스를 사용할 수 없다. "수학개론"이라는 과목이 있으면 잘못 매칭된다.

1NF로 변환:

```sql
CREATE TABLE student_courses (
    student_id   INT,
    name         VARCHAR(100),
    course       VARCHAR(100)
);

INSERT INTO student_courses VALUES
(1, '김철수', '수학'),
(1, '김철수', '영어'),
(1, '김철수', '물리'),
(2, '이영희', '영어'),
(2, '이영희', '화학');
```

하나의 행에 하나의 과목만 들어간다. 이제 `WHERE course = '수학'`으로 정확히 검색할 수 있다.

반복 그룹(repeating group)도 1NF 위반이다:

```sql
CREATE TABLE orders_bad (
    order_id    INT,
    product1    VARCHAR(100),
    quantity1   INT,
    product2    VARCHAR(100),
    quantity2   INT,
    product3    VARCHAR(100),
    quantity3   INT
);
```

상품이 4개 이상이면 컬럼을 추가해야 한다. 상품이 1개인 주문은 나머지 컬럼이 NULL이다. 전체 주문의 상품 수를 세려면 각 컬럼을 개별적으로 확인해야 한다. 이 구조도 행을 분리하여 1NF로 변환한다.

## 제2정규형 (2NF)

2NF의 조건: 1NF를 만족하면서, 모든 비키 속성이 기본키 전체에 완전 함수 종속되어야 한다. 기본키의 일부에만 종속되는 부분 함수 종속(partial dependency)이 없어야 한다.

기본키가 단일 컬럼이면 부분 종속이 발생할 수 없다. 2NF 위반은 복합키에서만 발생한다.

예시. 수강 기록 테이블:

```sql
CREATE TABLE enrollments (
    student_id   INT,
    course_id    INT,
    student_name VARCHAR(100),
    course_name  VARCHAR(100),
    grade        CHAR(2),
    PRIMARY KEY (student_id, course_id)
);

INSERT INTO enrollments VALUES
(1, 101, '김철수', '데이터베이스', 'A'),
(1, 102, '김철수', '알고리즘', 'B+'),
(2, 101, '이영희', '데이터베이스', 'A+');
```

기본키는 `(student_id, course_id)`다. 함수 종속 관계를 보면:

- `(student_id, course_id) -> grade`: 키 전체에 종속. 완전 함수 종속.
- `student_id -> student_name`: 키의 일부에 종속. 부분 함수 종속.
- `course_id -> course_name`: 키의 일부에 종속. 부분 함수 종속.

`student_name`은 `course_id`와 무관하게 `student_id`만으로 결정된다. 김철수의 이름은 어떤 과목을 듣든 동일하다. 그런데 김철수가 두 과목을 들으면 이름이 두 번 저장된다. 이름이 바뀌면 두 행 모두 수정해야 하고, 하나만 수정하면 같은 학생인데 이름이 다른 모순이 생긴다.

2NF로 변환. 부분 종속을 별도 테이블로 분리한다:

```sql
CREATE TABLE students (
    student_id   INT PRIMARY KEY,
    student_name VARCHAR(100)
);

CREATE TABLE courses (
    course_id    INT PRIMARY KEY,
    course_name  VARCHAR(100)
);

CREATE TABLE enrollments (
    student_id   INT,
    course_id    INT,
    grade        CHAR(2),
    PRIMARY KEY (student_id, course_id)
);
```

각 테이블에 하나의 사실만 저장된다. 학생 이름은 `students` 테이블에 한 번만 존재한다.

## 제3정규형 (3NF)

3NF의 조건: 2NF를 만족하면서, 비키 속성 간의 이행 함수 종속(transitive dependency)이 없어야 한다.

이행 함수 종속이란 `A -> B -> C`에서 A가 키이고, B와 C가 비키 속성일 때, C가 A에 직접 종속되는 것이 아니라 B를 거쳐 간접 종속되는 관계를 말한다.

예시:

```sql
CREATE TABLE employees (
    emp_id      INT PRIMARY KEY,
    emp_name    VARCHAR(100),
    dept_id     INT,
    dept_name   VARCHAR(100),
    dept_head   VARCHAR(100)
);

INSERT INTO employees VALUES
(1, '김철수', 10, '개발팀', '박부장'),
(2, '이영희', 10, '개발팀', '박부장'),
(3, '박민수', 20, '기획팀', '최팀장');
```

함수 종속 관계:

- `emp_id -> dept_id`: 직원이 정해지면 부서가 결정된다.
- `dept_id -> dept_name`: 부서 번호가 정해지면 부서명이 결정된다.
- `dept_id -> dept_head`: 부서 번호가 정해지면 부서장이 결정된다.

`dept_name`과 `dept_head`는 `emp_id`에 직접 종속되는 것이 아니라 `dept_id`를 거쳐 이행적으로 종속된다. 개발팀에 직원이 100명이면 "개발팀"이라는 이름과 "박부장"이라는 부서장 정보가 100번 중복 저장된다.

3NF로 변환:

```sql
CREATE TABLE departments (
    dept_id     INT PRIMARY KEY,
    dept_name   VARCHAR(100),
    dept_head   VARCHAR(100)
);

CREATE TABLE employees (
    emp_id      INT PRIMARY KEY,
    emp_name    VARCHAR(100),
    dept_id     INT
);
```

부서 정보가 `departments` 테이블에 한 번만 저장된다. 부서명이 바뀌면 한 행만 수정하면 된다.

## BCNF (Boyce-Codd Normal Form)

BCNF의 조건: 모든 결정자가 후보키(candidate key)여야 한다.

3NF는 비키 속성이 키에 완전 종속되는 것만 요구한다. BCNF는 더 강력하다. 함수 종속의 결정자(화살표 왼쪽)가 반드시 후보키여야 한다.

3NF를 만족하면서 BCNF를 위반하는 경우는 드물지만, 존재한다. 대표적인 예시가 수강 지도 테이블이다:

```sql
CREATE TABLE course_assignments (
    student_id   INT,
    course       VARCHAR(100),
    instructor   VARCHAR(100),
    PRIMARY KEY (student_id, course)
);
```

비즈니스 규칙:

- 한 학생은 같은 과목을 한 번만 수강한다. -> `(student_id, course)`가 기본키.
- 한 교수는 하나의 과목만 가르친다. -> `instructor -> course`.
- 같은 과목을 여러 교수가 가르칠 수 있다.

`instructor -> course`에서 `instructor`는 결정자이지만 후보키가 아니다. `instructor`만으로는 행을 유일하게 식별할 수 없기 때문이다(같은 교수가 여러 학생을 가르친다).

이 경우 교수가 담당 과목을 변경하면, 해당 교수에게 배정된 모든 학생의 행을 수정해야 한다.

BCNF로 변환:

```sql
CREATE TABLE instructors (
    instructor   VARCHAR(100) PRIMARY KEY,
    course       VARCHAR(100)
);

CREATE TABLE student_instructors (
    student_id   INT,
    instructor   VARCHAR(100),
    PRIMARY KEY (student_id, instructor)
);
```

결정자인 `instructor`가 `instructors` 테이블의 기본키(후보키)가 되었다. 교수와 과목의 관계는 한 곳에서만 관리된다.

## 정규화 과정 전체 예시

처음의 주문 테이블을 단계적으로 정규화해 보자:

```sql
-- 원본 (정규화 전)
CREATE TABLE orders (
    order_id    INT,
    customer    VARCHAR(100),
    product     VARCHAR(100),
    category    VARCHAR(50),
    price       DECIMAL(10,2),
    quantity    INT
);
```

함수 종속:

- `order_id -> customer` (주문이 정해지면 고객이 결정)
- `product -> category` (상품이 정해지면 카테고리가 결정)
- `product -> price` (상품이 정해지면 가격이 결정)

1NF는 이미 만족한다. 모든 컬럼이 원자값이다.

2NF 검사: 기본키가 `order_id` 단일 컬럼이라면 부분 종속이 없으므로 2NF도 만족한다. 하지만 실제로 한 주문에 여러 상품이 있을 수 있다면 `(order_id, product)`가 키가 되어야 하고, `customer`가 `order_id`에만 종속되므로 2NF 위반이다.

3NF 검사: `product -> category`는 이행 함수 종속이다. `order_id -> product -> category`로 category가 키에 간접 종속된다.

정규화 결과:

```sql
CREATE TABLE customers (
    customer_id  INT PRIMARY KEY AUTO_INCREMENT,
    name         VARCHAR(100)
);

CREATE TABLE products (
    product_id   INT PRIMARY KEY AUTO_INCREMENT,
    name         VARCHAR(100),
    category     VARCHAR(50),
    price        DECIMAL(10,2)
);

CREATE TABLE orders (
    order_id     INT PRIMARY KEY AUTO_INCREMENT,
    customer_id  INT,
    order_date   DATE
);

CREATE TABLE order_items (
    order_id     INT,
    product_id   INT,
    quantity     INT,
    PRIMARY KEY (order_id, product_id)
);
```

각 테이블이 하나의 사실을 담는다:

- `customers`: 고객 정보
- `products`: 상품 정보 (카테고리와 가격 포함)
- `orders`: 주문 메타 정보
- `order_items`: 어떤 주문에 어떤 상품이 몇 개 포함되었는지

이제 상품 가격을 바꾸면 `products` 테이블 한 행만 수정하면 된다. 아직 주문이 없는 상품도 등록할 수 있다. 주문을 삭제해도 상품 정보는 유지된다. 세 가지 이상이 모두 해소되었다.

## 과도한 정규화

4NF(다치 종속), 5NF(조인 종속)도 이론적으로 존재한다. 하지만 실무에서는 3NF 또는 BCNF까지만 적용하는 것이 일반적이다. 그 이상의 정규화는 테이블이 지나치게 분리되어 조인이 많아지고, 설계의 복잡도 대비 얻는 이득이 크지 않다.

정규화의 목적은 학술적 완벽함이 아니다. 데이터 무결성을 보장하면서도 실용적인 수준에서 중복을 제거하는 것이다. 3NF까지 정규화하면 대부분의 갱신 이상, 삽입 이상, 삭제 이상이 해소된다.

## 정규화의 진짜 목적

정규화는 성능 최적화가 아니다. 정규화된 구조가 오히려 성능에 불리할 수 있다. 조인이 많아지고, 단순한 조회에도 여러 테이블을 참조해야 한다.

정규화의 목적은 데이터 무결성이다. 하나의 사실이 하나의 장소에만 저장되면, 그 사실을 수정할 때 한 곳만 바꾸면 된다. 데이터가 모순되지 않는다. 이것이 정규화가 추구하는 핵심 가치다.

설계를 시작할 때는 정규화된 구조로 출발한다. 정규화된 구조는 변경에 유연하고, 예상치 못한 요구사항에 대응하기 쉽다. 성능 문제가 확인되면 그때 의도적으로 정규화를 깨는 것이 올바른 순서다. 이것이 반정규화이며, 다음 주제다.
