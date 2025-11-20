import groovy.transform.Field

@Library('jenkins-library@v6.0.1') _

gitflow {
    apply properties
}

goProjectPipeline("github.maicoin.site/maiami/go-ethereum") {
    kubernetes {
        cloud 'kubernetes'
        yamlFile "jenkins/go-agent.yaml"
        defaultContainer "builder"
    }

    buildDiscarder {
        numToKeepStr '5'
        daysToKeepStr '7'
    }

    install {
        echo "Codebase pulled."
    }

    tests {
//         parallel {
//             suite("lint") {
//                 withGoModuleLocal() {
//                     sh "go run build/ci.go lint"
//                 }
//             }
//
//             suite("build") {
//                 withGoModuleLocal() {
//                     sh "git submodule update --init --recursive"
//                     sh "go run build/ci.go test"
//                 }
//             }
//         }
    }

    dockerImages {
         dockerImage {
            image "quay.io/amis/indexer-geth"

            credential "${env.QUAY_IO_CREDENTIAL_ID}"


//             when {
//                 anyOf {
//                     env.TAG_NAME =~ /^v(\d+.)+\d+/
//                 }
//             }
         }
    }
}

// vim:filetype=groovy:foldmethod=marker:foldlevel=1:
