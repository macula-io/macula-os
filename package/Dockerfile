FROM scratch
COPY build/macula/system/ /macula/system/
ENV PATH /macula/system/macula/current:/macula/system/k3s/current:${PATH}
ENTRYPOINT ["maculaos"]
CMD ["help"]
