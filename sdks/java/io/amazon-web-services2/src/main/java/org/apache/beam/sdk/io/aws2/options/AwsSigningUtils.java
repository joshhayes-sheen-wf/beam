package org.apache.beam.sdk.io.aws2.options;

import org.apache.http.HttpRequestInterceptor;
import software.amazon.awssdk.auth.credentials.AwsCredentialsProvider;

import org.apache.beam.sdk.io.elasticsearch.AuthorizationInterceptorProvider;

public class AWSSigningAuthorizationInterceptorProvider implements AuthorizationInterceptorProvider {

    private final String servicename;
    private final String region;
    private final String awsCredentialsProviderSerialized;

    public AWSSigningAuthorizationInterceptorProvider(String servicename, String region, AwsCredentialsProvider credentialsProvider) {
        this.awsCredentialsProviderSerialized =
                AwsSerializableUtils.serializeAwsCredentialsProvider(credentialsProvider);
        this.servicename = servicename;
        this.region = region;
    }

    @Override
    public HttpRequestInterceptor getAuthorizationRequestInterceptor() {
        AWS4Signer signer = new AWS4Signer();
        signer.setServiceName(this.servicename);
        signer.setRegionName(this.region);
        return new AWSRequestSigningApacheInterceptor(this.servicename, signer,
                AwsSerializableUtils.deserializeAwsCredentialsProvider(this.awsCredentialsProviderSerialized));
    }
}