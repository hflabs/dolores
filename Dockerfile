FROM adoptopenjdk/maven-openjdk11

ARG CUSTOMER_NAME
ARG CORE_REVISION
ARG CUSTOMER_REVISION
ARG JDBC_USERNAME
ARG FACTOR_BUILD_FILTER

COPY settings_hflabs.xml /opt/.m2/

RUN apt-get update && apt-get install -y git

# CDI в коде хранится в двух репозиториях — core (общая часть) + сборка заказчика
# clone core
WORKDIR /opt
RUN git clone -n http://automation:pass666@gitlab.test.ru/test/test.git

# build core
WORKDIR /opt/test
RUN git checkout ${CORE_REVISION} && \
    mvn install -s /opt/.m2/settings_hflabs.xml -Dmaven.test.skip=true

# clone customer
WORKDIR /opt
RUN git clone -n http://automation:pass666@gitlab.hflabs.ru/test/test-${CUSTOMER_NAME}.git

# build and run customer
WORKDIR /opt/test-${CUSTOMER_NAME}
RUN git checkout ${CUSTOMER_REVISION}

ENV JDBC_USERNAME=${JDBC_USERNAME}
ENV FACTOR_BUILD_FILTER=${FACTOR_BUILD_FILTER}
CMD mvn clean install --no-snapshot-updates -s /opt/.m2/settings_hflabs.xml \
        -Dmaven.test.skip=true \
        -Dmaven.test.mats.skip=false \
        -Dteamcity.factor.build.filter=tag:${FACTOR_BUILD_FILTER} \
        -Dteamcity.username=automation \
        -Dteamcity.password=pass666 \
        -Djdbc.username=${JDBC_USERNAME} \
        -Dmats.sleepBeforeTestsAfterServersStartedInSec=1000000 \
        -Dmats.sleepBeforeTestsAfterServersStartedInHours=240 \
        -Djboss.JAVA_OPTS_JVM="-server -Xms1g -Xmx22g -XX:-UseCodeCacheFlushing -XX:ReservedCodeCacheSize=256m -XX:MaxMetaspaceSize=512m -XX:-OmitStackTraceInFastThrow -XX:+UseCompressedOops"

EXPOSE 8080 18080 9990 19990 5005
